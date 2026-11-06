---
page_title: "msexchangeonline_role_group (List Resource) - msexchangeonline"
subcategory: ""
description: |-
  Lists the existing msexchangeonline_role_group objects, for terraform query.
---

# msexchangeonline_role_group (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeonline_role_group`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/role_group) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing tenant](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeonline_role_group" "all" {
  provider = msexchangeonline
}
```
