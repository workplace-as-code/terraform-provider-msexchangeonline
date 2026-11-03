---
page_title: "msexchangeonline_mailbox_plan (List Resource) - msexchangeonline"
subcategory: ""
description: |-
  Lists the existing msexchangeonline_mailbox_plan objects, for terraform query.
---

# msexchangeonline_mailbox_plan (List Resource)

Lists the existing objects of this type, built-in ones included, for `terraform query` (Terraform 1.14 and later). The list block takes no arguments.

Each result is an object that the [`msexchangeonline_mailbox_plan`](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/resources/mailbox_plan) resource can import. `terraform query -generate-config-out=generated.tf` writes the resource and `import` blocks of every result: see [Import an existing tenant](https://registry.terraform.io/providers/workplace-as-code/msexchangeonline/latest/docs/guides/import_existing).

## Example Usage

```terraform
list "msexchangeonline_mailbox_plan" "all" {
  provider = msexchangeonline
}
```
