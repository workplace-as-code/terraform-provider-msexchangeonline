---
page_title: "msexchangeonline_tenant_allow_block_list_item (List Resource) - msexchangeonline"
subcategory: ""
description: |-
  Lists the existing msexchangeonline_tenant_allow_block_list_item objects, for terraform query.
---

# msexchangeonline_tenant_allow_block_list_item (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeonline_tenant_allow_block_list_item`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/tenant_allow_block_list_item) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing tenant](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeonline_tenant_allow_block_list_item" "all" {
  provider = msexchangeonline
}
```
