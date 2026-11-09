package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// test/parameters.txt lists "<cmdlet> <parameter>" for every attribute of the
// provider: test/preflight.ps1 checks each line on the tenant before the
// integration tests change anything.

const (
	parametersFile     = "../../test/parameters.txt"
	generatedHeader    = "# generated (do not edit)"
	handWrittenHeader  = "# hand-written resources"
	bridgeScriptsDir   = "../../bridge/scripts"
	updateParametersEn = "UPDATE_PARAMETERS"
)

// generatedParameterLines returns the lines the generic specs give: the
// cmdlets exo_object.ps1 and exo_rule.ps1 pass each field to.
func generatedParameterLines() []string {
	seen := map[string]bool{}
	for _, spec := range objectSpecs {
		for _, f := range spec.fields {
			create := !spec.singleton && !spec.adopted
			update := !f.forceNew
			if strings.HasSuffix(spec.noun, "Rule") {
				// Enable-/Disable- change the state of an existing rule.
				update = update && f.param != "Enabled"
			}
			if spec.script == "exo_rule.ps1" {
				// exo_rule.ps1 sets the priority once the rule exists.
				create = create && f.param != "Priority"
			}
			if create {
				seen["New-"+spec.noun+" "+f.param] = true
			}
			if update {
				seen["Set-"+spec.noun+" "+f.param] = true
			}
		}
	}
	lines := make([]string, 0, len(seen))
	for l := range seen {
		lines = append(lines, l)
	}
	sort.Strings(lines)
	return lines
}

// splitParametersFile returns the two sections of the file.
func splitParametersFile(data string) (generated, handWritten []string, err error) {
	var current *[]string
	for _, line := range strings.Split(data, "\n") {
		switch {
		case line == generatedHeader:
			current = &generated
		case line == handWrittenHeader:
			current = &handWritten
		case line == "":
		case current == nil:
			return nil, nil, fmt.Errorf("line %q comes before the first section", line)
		default:
			*current = append(*current, line)
		}
	}
	if current == nil {
		return nil, nil, fmt.Errorf("no %q section", generatedHeader)
	}
	return generated, handWritten, nil
}

// checkHandWritten reports the lines whose cmdlet is in no bridge script, or
// whose parameter is not in the script that calls the cmdlet.
func checkHandWritten(lines []string, scriptsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(scriptsDir, "*.ps1"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no bridge scripts in %s: %v", scriptsDir, err)
	}
	scripts := make([]string, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, string(data))
	}
	var problems []string
	for _, line := range lines {
		cmdlet, param, ok := strings.Cut(line, " ")
		if !ok {
			problems = append(problems, fmt.Sprintf("%q is not <cmdlet> <parameter>", line))
			continue
		}
		cmdletRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(cmdlet) + `\b`)
		paramRe := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(param) + `\b`)
		found, hasParam := false, false
		for _, s := range scripts {
			if cmdletRe.MatchString(s) {
				found = true
				hasParam = hasParam || paramRe.MatchString(s)
			}
		}
		switch {
		case !found:
			problems = append(problems, line+": no bridge script calls this cmdlet")
		case !hasParam:
			problems = append(problems, line+": the script calling this cmdlet does not use this parameter")
		}
	}
	return problems, nil
}

func TestParametersFile(t *testing.T) {
	want := generatedParameterLines()
	data, err := os.ReadFile(parametersFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var handWritten []string
	if err == nil {
		_, handWritten, err = splitParametersFile(string(data))
		if err != nil {
			t.Fatal(err)
		}
	}
	if os.Getenv(updateParametersEn) != "" {
		content := generatedHeader + "\n" + strings.Join(want, "\n") + "\n" + handWrittenHeader + "\n"
		if len(handWritten) > 0 {
			content += strings.Join(handWritten, "\n") + "\n"
		}
		if err := os.WriteFile(parametersFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if data == nil {
		t.Fatalf("%s does not exist: run %s=1 go test ./internal/resources -run TestParametersFile", parametersFile, updateParametersEn)
	}
	got, _, err := splitParametersFile(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the generated section of %s is stale: run %s=1 go test ./internal/resources -run TestParametersFile", parametersFile, updateParametersEn)
	}
}

func TestParametersFileHandWritten(t *testing.T) {
	data, err := os.ReadFile(parametersFile)
	if err != nil {
		t.Fatal(err)
	}
	_, handWritten, err := splitParametersFile(string(data))
	if err != nil {
		t.Fatal(err)
	}
	problems, err := checkHandWritten(handWritten, bridgeScriptsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}

	// The check itself: a parameter no script uses is reported.
	problems, err = checkHandWritten([]string{"Set-ManagementRole Bogus"}, bridgeScriptsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "Set-ManagementRole Bogus") {
		t.Errorf("a parameter no script uses must be reported, got %v", problems)
	}
}
