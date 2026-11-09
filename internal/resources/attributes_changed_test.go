package resources

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// test/attributes_not_changed.txt lists the attributes that can change in place
// but that the integration tests never change. Each release shortens it; no new
// attribute may join it.

const attributesNotChangedFile = "../../test/attributes_not_changed.txt"

// replacesResource reports whether the attribute has a plan modifier that
// recreates the resource. The modifiers of every attribute type share a
// Description method and the field has the same name, hence reflection.
func replacesResource(attribute any) bool {
	field := reflect.ValueOf(attribute).FieldByName("PlanModifiers")
	if !field.IsValid() {
		return false
	}
	for i := 0; i < field.Len(); i++ {
		m, ok := field.Index(i).Interface().(interface {
			Description(context.Context) string
		})
		if ok && strings.Contains(m.Description(context.Background()), "destroy and recreate") {
			return true
		}
	}
	return false
}

// updatableAttributes returns "<type>.<attribute>" for every configurable
// attribute of every registered resource that can change in place.
func updatableAttributes() map[string]bool {
	out := map[string]bool{}
	for _, reg := range registrations() {
		r := reg.newResource()
		var meta resource.MetadataResponse
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "msexchangeonline"}, &meta)
		typeName := strings.TrimPrefix(meta.TypeName, "msexchangeonline_")
		var schema resource.SchemaResponse
		r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
		for name, a := range schema.Schema.Attributes {
			if name != "id" && (a.IsOptional() || a.IsRequired()) && !replacesResource(a) {
				out[typeName+"."+name] = true
			}
		}
	}
	return out
}

var (
	localDefinitionRe = regexp.MustCompile(`^\s*(\w+)\s*=\s*(.*)$`)
	localReferenceRe  = regexp.MustCompile(`\blocal\.(\w+)`)
	resourceBlockRe   = regexp.MustCompile(`^resource "msexchangeonline_(\w+)" "\w+" \{`)
	writeRestoreRe    = regexp.MustCompile(`write_and_restore\s+'?msexchangeonline_(\w+)\.[^\s']+'?\s+(\w+)`)
)

// changedAttributes returns "<type>.<attribute>" for every attribute the
// integration tests change: its value depends on the phase of the run, in the
// configuration, or run.sh writes it with write_and_restore.
func changedAttributes(tf, runSh string) map[string]bool {
	lines := strings.Split(tf, "\n")

	// The locals that depend on the phase, directly or through another local.
	phaseLocals := map[string]bool{}
	dependsOnPhase := func(expr string) bool {
		if strings.Contains(expr, "var.phase") {
			return true
		}
		for _, m := range localReferenceRe.FindAllStringSubmatch(expr, -1) {
			if phaseLocals[m[1]] {
				return true
			}
		}
		return false
	}
	for grown := true; grown; {
		grown = false
		inLocals := false
		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "locals {"):
				inLocals = true
			case line == "}":
				inLocals = false
			case inLocals:
				if m := localDefinitionRe.FindStringSubmatch(line); m != nil && !phaseLocals[m[1]] && dependsOnPhase(m[2]) {
					phaseLocals[m[1]] = true
					grown = true
				}
			}
		}
	}

	out := map[string]bool{}
	typeName, attribute := "", ""
	depth := 0
	for _, line := range lines {
		if m := resourceBlockRe.FindStringSubmatch(line); m != nil {
			typeName, attribute, depth = m[1], "", 1
			continue
		}
		if typeName == "" {
			continue
		}
		if depth == 1 {
			attribute = ""
			if m := localDefinitionRe.FindStringSubmatch(line); m != nil {
				attribute = m[1]
			}
		}
		if attribute != "" && dependsOnPhase(line) {
			out[typeName+"."+attribute] = true
		}
		depth += strings.Count(line, "{") + strings.Count(line, "[") + strings.Count(line, "(") -
			strings.Count(line, "}") - strings.Count(line, "]") - strings.Count(line, ")")
		if depth == 0 {
			typeName = ""
		}
	}
	for _, m := range writeRestoreRe.FindAllStringSubmatch(runSh, -1) {
		out[m[1]+"."+m[2]] = true
	}
	return out
}

func TestAttributesNeverChanged(t *testing.T) {
	files, err := filepath.Glob("../../test/integration/*.tf")
	if err != nil || len(files) == 0 {
		t.Fatalf("no integration configuration found: %v", err)
	}
	var tf strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		tf.Write(data)
		tf.WriteString("\n")
	}
	runSh, err := os.ReadFile("../../test/run.sh")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	if data, err := os.ReadFile(attributesNotChangedFile); err == nil {
		for _, l := range strings.Fields(string(data)) {
			listed[l] = true
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	changed := changedAttributes(tf.String(), string(runSh))
	updatable := updatableAttributes()
	var problems []string
	for a := range updatable {
		if !changed[a] && !listed[a] {
			problems = append(problems, a+": the integration tests never change it")
		}
	}
	for a := range listed {
		switch {
		case !updatable[a]:
			problems = append(problems, a+": listed but it is not an attribute that changes in place")
		case changed[a]:
			problems = append(problems, a+": listed but the integration tests change it, remove it from the list")
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

func TestAttributesChangedDetection(t *testing.T) {
	const tf = `
locals {
  updated = var.phase == "update"
  comment = local.updated ? "updated" : "created"
}

resource "msexchangeonline_safe_links_policy" "test" {
  name         = "TF-CI"
  track_clicks = !local.updated
  nested = {
    inner = local.updated
  }
}

resource "msexchangeonline_transport_rule" "test" {
  comments = local.comment
}

resource "msexchangeonline_cross_tenant_partner" "test" {
  capabilities = concat(
    [],
    local.updated ? ["a"] : [],
  )
}
`
	const runSh = `write_and_restore msexchangeonline_transport_config.this reply_all_storm_block_duration_hours \
  write_and_restore 'msexchangeonline_ome_configuration.default[0]' otp_enabled \
`
	got := changedAttributes(tf, runSh)
	for _, want := range []string{
		"safe_links_policy.track_clicks", "safe_links_policy.nested", "transport_rule.comments",
		"cross_tenant_partner.capabilities", "transport_config.reply_all_storm_block_duration_hours",
		"ome_configuration.otp_enabled",
	} {
		if !got[want] {
			t.Errorf("%s must count as changed, got %v", want, got)
		}
	}
	if got["safe_links_policy.name"] {
		t.Error("a constant must not count as changed")
	}
}
