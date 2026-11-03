---
page_title: "msexchangeonline_activesync_organization_settings (List Resource) - msexchangeonline"
subcategory: ""
description: |-
  Lists the existing msexchangeonline_activesync_organization_settings objects, for terraform query.
---

# msexchangeonline_activesync_organization_settings (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeonline_activesync_organization_settings`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/activesync_organization_settings) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing tenant](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeonline_activesync_organization_settings" "all" {
  provider = msexchangeonline
}
```
