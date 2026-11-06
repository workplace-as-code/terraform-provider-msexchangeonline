package resources

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
)

// Parameter names are the ones of the Set-<noun> cmdlet (checked against the
// ExchangeOnlineManagement module), not guesses: a wrong name fails every apply.
var objectSpecs = []objectSpec{
	{
		typeName: "retention_policy_tag",
		noun:     "RetentionPolicyTag",
		description: "Manages a retention policy tag (messaging records management), which sets how long items are kept " +
			"and what happens when the age limit is reached. Link tags to mailboxes with `msexchangeonline_retention_policy`",
		notes: "`type` is required to create a tag.",
		fields: []field{
			{
				name: "type", param: "Type", kind: stringField, forceNew: true,
				desc: "Folder the tag applies to: `All` (default policy tag), `Personal` (users apply it themselves), or a default folder " +
					"such as `Inbox`, `SentItems`, `DeletedItems`, `JunkEmail`, `Calendar`, `Tasks`, `RecoverableItems`. " +
					"Changing it creates a new tag.",
			},
			daysAttr("age_limit_for_retention", "AgeLimitForRetention", "Age, in days, at which `retention_action` applies. "+
				"`0` means no age limit, which Exchange only accepts with `retention_enabled = false` (built-in tags such as Never Delete).",
				int64validator.Between(0, 24855)),
			enumAttr("retention_action", "RetentionAction", "Action when the age limit is reached.",
				"DeleteAndAllowRecovery", "PermanentlyDelete", "MoveToArchive", "MarkAsPastRetentionLimit"),
			boolAttr("retention_enabled", "RetentionEnabled", "Whether the tag is applied. `false` keeps it visible but inactive."),
			stringAttr("comment", "Comment", "Comment shown to users in Outlook."),
		},
	},
	{
		typeName:  "hosted_connection_filter_policy",
		noun:      "HostedConnectionFilterPolicy",
		singleton: true,
		identity:  "Default",
		description: "Manages the connection filter policy, the first layer of Exchange Online Protection: IP addresses whose mail is " +
			"always accepted or always blocked. Exchange Online has a single connection filter policy, `Default`",
		fields: []field{
			listAttr("ip_allow_list", "IPAllowList", "IP addresses or CIDR ranges (/24 to /32) whose mail skips spam filtering."),
			listAttr("ip_block_list", "IPBlockList", "IP addresses or CIDR ranges (/24 to /32) whose mail is always rejected."),
			boolAttr("enable_safe_list", "EnableSafeList", "Also skip spam filtering for senders on the Microsoft safe list."),
		},
	},
	{
		typeName:    "external_in_outlook",
		noun:        "ExternalInOutlook",
		singleton:   true,
		description: "Manages the External tag that Outlook shows on messages from senders outside the organization",
		fields: []field{
			boolAttr("enabled", "Enabled", "Show the External tag."),
			listAttr("allow_list", "AllowList", "Email addresses and domains that never get the External tag."),
		},
	},
	{
		typeName:    "transport_config",
		noun:        "TransportConfig",
		singleton:   true,
		description: "Manages the organization-wide transport settings",
		fields: []field{
			boolAttr("smtp_client_authentication_disabled", "SmtpClientAuthenticationDisabled",
				"Disable SMTP AUTH client submission (port 587) for the whole organization. Mailboxes can still enable it individually."),
			boolAttr("allow_legacy_tls_clients", "AllowLegacyTLSClients", "Allow TLS 1.0 and 1.1 clients to connect with SMTP AUTH."),
			stringAttr("external_postmaster_address", "ExternalPostmasterAddress", "Sender address of the delivery status notifications sent to external senders."),
			boolAttr("reply_all_storm_protection_enabled", "ReplyAllStormProtectionEnabled", "Block reply-all storms."),
			int64Attr("reply_all_storm_detection_minimum_recipients", "ReplyAllStormDetectionMinimumRecipients",
				"Minimum number of recipients for a thread to be considered a reply-all storm.", int64validator.Between(1000, 5000)),
			int64Attr("reply_all_storm_detection_minimum_replies", "ReplyAllStormDetectionMinimumReplies",
				"Minimum number of reply-all messages, within 60 minutes, to detect a storm.", int64validator.Between(5, 20)),
			int64Attr("reply_all_storm_block_duration_hours", "ReplyAllStormBlockDurationHours",
				"Hours during which further reply-all messages are blocked.", int64validator.Between(1, 12)),
			boolAttr("address_book_policy_routing_enabled", "AddressBookPolicyRoutingEnabled",
				"Resolve recipients to the global address list of the sender's address book policy."),
			boolAttr("convert_disclaimer_wrapper_to_eml", "ConvertDisclaimerWrapperToEml",
				"Attach the original message as an .eml file when a transport rule disclaimer cannot be inserted."),
			boolAttr("rfc2231_encoding_enabled", "Rfc2231EncodingEnabled", "Encode attachment names with RFC 2231."),
			stringAttr("journaling_report_ndr_to", "JournalingReportNdrTo",
				"Address that receives the journal reports that cannot be delivered, required before the first journal rule is created. "+
					"It cannot be an Exchange Online mailbox. Once set, it can be changed but never cleared."),
		},
	},
	{
		typeName:    "admin_audit_log_config",
		noun:        "AdminAuditLogConfig",
		singleton:   true,
		description: "Manages the recording of user and admin activities in the Microsoft 365 unified audit log",
		fields: []field{
			boolAttr("unified_audit_log_ingestion_enabled", "UnifiedAuditLogIngestionEnabled",
				"Record user and admin activities in the unified audit log. A change can take up to 60 minutes to apply."),
		},
	},
	{
		typeName:    "activesync_organization_settings",
		noun:        "ActiveSyncOrganizationSettings",
		singleton:   true,
		description: "Manages the Exchange ActiveSync settings of the organization: what happens to mobile devices that match no device access rule",
		fields: []field{
			enumAttr("default_access_level", "DefaultAccessLevel", "Access of the devices that match no "+
				"`msexchangeonline_activesync_device_access_rule`.", "Allow", "Block", "Quarantine"),
			listAttr("admin_mail_recipients", "AdminMailRecipients", "Email addresses notified when a device is quarantined."),
			stringAttr("user_mail_insert", "UserMailInsert", "Text added to the message sent to users whose device is quarantined, "+
				"up to 256 characters."),
			boolAttr("allow_access_for_unsupported_platform", "AllowAccessForUnSupportedPlatform",
				"Let devices that do not support ActiveSync policies connect."),
		},
	},
	{
		typeName: "mailbox_plan",
		noun:     "MailboxPlan",
		script:   "mailbox_plan.ps1",
		adopted:  true,
		description: "Manages a mailbox plan: the size limits and deleted item retention that new mailboxes get from " +
			"their license. Existing mailboxes keep their settings",
		notes: "`name` is the display name of the plan, the same in every tenant, for example `ExchangeOnlineEnterprise` " +
			"(Exchange Online Plan 2), `ExchangeOnline` (Plan 1), `ExchangeOnlineDeskless` (Kiosk) or `ExchangeOnlineEssentials`. " +
			"Sizes are in megabytes and cannot exceed what the license allows.",
		fields: []field{
			int64Attr("max_send_size_mb", "MaxSendSize", "Largest message a new mailbox can send, in MB (up to 150).",
				int64validator.Between(1, 150)),
			int64Attr("max_receive_size_mb", "MaxReceiveSize", "Largest message a new mailbox can receive, in MB (up to 150).",
				int64validator.Between(1, 150)),
			int64Attr("issue_warning_quota_mb", "IssueWarningQuota", "Mailbox size, in MB, at which the user is warned.",
				int64validator.AtLeast(1)),
			int64Attr("prohibit_send_quota_mb", "ProhibitSendQuota", "Mailbox size, in MB, at which the user can no longer send.",
				int64validator.AtLeast(1)),
			int64Attr("prohibit_send_receive_quota_mb", "ProhibitSendReceiveQuota",
				"Mailbox size, in MB, at which the mailbox can no longer send or receive.", int64validator.AtLeast(1)),
			daysAttr("retain_deleted_items_for_days", "RetainDeletedItemsFor", "Days deleted items can be recovered (up to 30).",
				int64validator.Between(1, 30)),
			stringAttr("retention_policy", "RetentionPolicy", "Retention policy of new mailboxes, by name."),
		},
	},
	{
		typeName: "cas_mailbox_plan",
		noun:     "CASMailboxPlan",
		script:   "mailbox_plan.ps1",
		adopted:  true,
		description: "Manages a CAS mailbox plan: the client protocols that new mailboxes get from their license. " +
			"Existing mailboxes keep their settings",
		notes: "`name` is the display name of the plan, the same in every tenant, for example `ExchangeOnlineEnterprise` " +
			"(Exchange Online Plan 2), `ExchangeOnline` (Plan 1), `ExchangeOnlineDeskless` (Kiosk) or `ExchangeOnlineEssentials`.",
		fields: []field{
			boolAttr("pop_enabled", "PopEnabled", "Allow POP3."),
			boolAttr("imap_enabled", "ImapEnabled", "Allow IMAP4."),
			boolAttr("active_sync_enabled", "ActiveSyncEnabled", "Allow Exchange ActiveSync."),
			boolAttr("ews_enabled", "EwsEnabled", "Allow Exchange Web Services."),
			boolAttr("mapi_enabled", "MAPIEnabled", "Allow MAPI clients such as Outlook for Windows."),
			boolAttr("owa_enabled", "OWAEnabled", "Allow Outlook on the web."),
			stringAttr("owa_mailbox_policy", "OwaMailboxPolicy", "Outlook on the web mailbox policy, by name."),
		},
	},
	{
		typeName: "sharing_policy",
		noun:     "SharingPolicy",
		script:   "sharing_policy.ps1",
		description: "Manages a sharing policy, which sets the calendar and contact information users can share with people " +
			"outside the organization",
		notes: "`domains` is required to create a policy. Mailboxes without a sharing policy of their own use the " +
			"default one, `Default Sharing Policy`: import it to restrict sharing for the whole organization.",
		fields: []field{
			setAttr("domains", "Domains", "Domains and what users can share with them, as `domain:Action` or "+
				"`domain:Action,Action` without spaces. The domain is a domain name (subdomains are not included), `*` "+
				"for every federated organization or `Anonymous` for anyone with a link. Actions: "+
				"`CalendarSharingFreeBusySimple`, `CalendarSharingFreeBusyDetail`, `CalendarSharingFreeBusyReviewer`, "+
				"`ContactsSharing`."),
			boolAttr("enabled", "Enabled", "Whether the policy applies. Mailboxes keep sharing until the sharing "+
				"policy assistant next processes them."),
		},
	},
	{
		typeName: "organization_relationship",
		noun:     "OrganizationRelationship",
		description: "Manages an organization relationship, which shares free/busy information, MailTips and photos " +
			"with another Microsoft 365 organization or an on-premises Exchange organization",
		notes: "`domain_names` is required to create a relationship. Get the `target_*` values of the other organization with " +
			"`Get-FederationInformation -DomainName <domain>`. The mailbox move settings of cross-tenant migrations are not managed.",
		fields: []field{
			setAttr("domain_names", "DomainNames", "Domains of the other organization."),
			boolAttr("enabled", "Enabled", "Whether the relationship is used."),
			boolAttr("free_busy_access_enabled", "FreeBusyAccessEnabled", "Share free/busy information with the other organization."),
			enumAttr("free_busy_access_level", "FreeBusyAccessLevel", "Free/busy details shared when `free_busy_access_enabled` is `true`.",
				"None", "AvailabilityOnly", "LimitedDetails"),
			boolAttr("mail_tips_access_enabled", "MailTipsAccessEnabled", "Share MailTips with the other organization."),
			enumAttr("mail_tips_access_level", "MailTipsAccessLevel", "MailTips shared when `mail_tips_access_enabled` is `true`.",
				"None", "Limited", "All"),
			boolAttr("photos_enabled", "PhotosEnabled", "Share user photos with the other organization."),
			boolAttr("archive_access_enabled", "ArchiveAccessEnabled", "The other organization hosts the cloud archives of "+
				"this organization's mailboxes (hybrid)."),
			boolAttr("delivery_report_enabled", "DeliveryReportEnabled", "Share delivery reports with the other organization."),
			stringAttr("organization_contact", "OrganizationContact", "Email address of a contact in the other organization."),
			stringAttr("target_application_uri", "TargetApplicationUri", "Application URI of the other organization, "+
				"for example `outlook.com` for Microsoft 365."),
			stringAttr("target_autodiscover_epr", "TargetAutodiscoverEpr", "Autodiscover endpoint of the other organization."),
			stringAttr("target_sharing_epr", "TargetSharingEpr", "Exchange Web Services endpoint of the other organization, "+
				"when Autodiscover cannot find it."),
			stringAttr("target_owa_url", "TargetOwaURL", "Outlook on the web URL of the other organization."),
		},
	},
	{
		typeName: "hosted_outbound_spam_filter_rule",
		noun:     "HostedOutboundSpamFilterRule",
		script:   "exo_rule.ps1",
		description: "Manages a standalone outbound spam filter rule, which applies an outbound spam filter policy to a set of senders. " +
			"Use it with a `msexchangeonline_outbound_spam_filter_policy` that has no sender condition of its own",
		notes: "`hosted_outbound_spam_filter_policy` and at least one of `from`, `from_member_of` or `sender_domain_is` are required to create a rule. " +
			"A condition left out of the configuration is not managed: set it to `[]` to clear it.",
		fields: []field{
			stringAttr("hosted_outbound_spam_filter_policy", "HostedOutboundSpamFilterPolicy", "Name of the outbound spam filter policy the rule applies."),
			int64Attr("priority", "Priority", "Priority of the rule. `0` is evaluated first.", int64validator.AtLeast(0)),
			boolAttr("enabled", "Enabled", "Whether the rule is enabled."),
			stringAttr("comments", "Comments", "Comments about the rule."),
			listAttr("from", "From", "Condition: the sender is one of these addresses or mailboxes."),
			listAttr("from_member_of", "FromMemberOf", "Condition: the sender is a member of one of these groups."),
			listAttr("sender_domain_is", "SenderDomainIs", "Condition: the sender's domain is one of these domains."),
			listAttr("except_if_from", "ExceptIfFrom", "Exception: the sender is one of these addresses or mailboxes."),
			listAttr("except_if_from_member_of", "ExceptIfFromMemberOf", "Exception: the sender is a member of one of these groups."),
			listAttr("except_if_sender_domain_is", "ExceptIfSenderDomainIs", "Exception: the sender's domain is one of these domains."),
		},
	},
	{
		typeName:    "journal_rule",
		noun:        "JournalRule",
		script:      "journal_rule.ps1",
		description: "Manages a journal rule, which sends a copy of the matching messages to an archive outside Exchange Online",
		notes: "`journal_email_address` is required to create a rule. Set `transport_config.journaling_report_ndr_to` first: " +
			"Exchange Online refuses to create a rule without it, and sends there the journal reports it cannot deliver.",
		fields: []field{
			stringAttr("journal_email_address", "JournalEmailAddress",
				"Address that receives the journal reports: an archiving system or service outside Exchange Online, never an Exchange Online mailbox."),
			stringAttr("recipient", "Recipient",
				"SMTP address of the mailbox, contact or group whose messages are journaled. `\"\"` journals the messages of every recipient."),
			enumAttr("scope", "Scope", "Messages the rule journals: all of them, internal ones or external ones.", "Global", "Internal", "External"),
			boolAttr("enabled", "Enabled", "Whether the rule is enabled."),
		},
	},
	{
		typeName:    "eop_protection_policy_rule",
		noun:        "EOPProtectionPolicyRule",
		script:      "eop_protection_policy_rule.ps1",
		description: "Manages the Exchange Online Protection part of a preset security policy (Standard or Strict): whether it applies, and to which recipients",
		notes: "`name` is `Standard Preset Security Policy` or `Strict Preset Security Policy`. The rule is not created: it exists once the Exchange Online Protection part of the preset has been applied to recipients in the Microsoft Defender portal, and the resource adopts it. " +
			"Destroying the resource turns the preset off and keeps the rule. A condition left out of the configuration is not managed: set it to `[]` to clear it.",
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the preset security policy applies. Turning it off keeps the rule and its recipients."),
			stringAttr("comments", "Comments", "Comments about the rule."),
			listAttr("sent_to", "SentTo", "Condition: the recipient is one of these mailboxes, mail users or contacts."),
			listAttr("sent_to_member_of", "SentToMemberOf", "Condition: the recipient is a member of one of these groups."),
			listAttr("recipient_domain_is", "RecipientDomainIs", "Condition: the recipient's domain is one of these accepted domains."),
			listAttr("except_if_sent_to", "ExceptIfSentTo", "Exception: the recipient is one of these mailboxes, mail users or contacts."),
			listAttr("except_if_sent_to_member_of", "ExceptIfSentToMemberOf", "Exception: the recipient is a member of one of these groups."),
			listAttr("except_if_recipient_domain_is", "ExceptIfRecipientDomainIs", "Exception: the recipient's domain is one of these accepted domains."),
		},
	},
	{
		typeName:    "atp_protection_policy_rule",
		noun:        "ATPProtectionPolicyRule",
		script:      "atp_protection_policy_rule.ps1",
		description: "Manages the Defender for Office 365 part of a preset security policy (Standard or Strict): whether Safe Links and Safe Attachments protection applies, and to which recipients. Needs Defender for Office 365",
		notes: "`name` is `Standard Preset Security Policy` or `Strict Preset Security Policy`. The rule is not created: it exists once the Defender for Office 365 part of the preset has been applied to recipients in the Microsoft Defender portal, and the resource adopts it. " +
			"Destroying the resource turns the preset off and keeps the rule. A condition left out of the configuration is not managed: set it to `[]` to clear it.",
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the preset security policy applies. Turning it off keeps the rule and its recipients."),
			stringAttr("comments", "Comments", "Comments about the rule."),
			listAttr("sent_to", "SentTo", "Condition: the recipient is one of these mailboxes, mail users or contacts."),
			listAttr("sent_to_member_of", "SentToMemberOf", "Condition: the recipient is a member of one of these groups."),
			listAttr("recipient_domain_is", "RecipientDomainIs", "Condition: the recipient's domain is one of these accepted domains."),
			listAttr("except_if_sent_to", "ExceptIfSentTo", "Exception: the recipient is one of these mailboxes, mail users or contacts."),
			listAttr("except_if_sent_to_member_of", "ExceptIfSentToMemberOf", "Exception: the recipient is a member of one of these groups."),
			listAttr("except_if_recipient_domain_is", "ExceptIfRecipientDomainIs", "Exception: the recipient's domain is one of these accepted domains."),
		},
	},
	{
		typeName: "address_list",
		noun:     "AddressList",
		script:   "address_book.ps1",
		description: "Manages an address list: the recipients that match a filter, shown to users through an address book " +
			"policy",
		notes: "In Exchange Online, users only see an address list through an address book policy. Exchange Online cannot " +
			"recalculate the members of a list: after a change of `recipient_filter`, a recipient enters or leaves the list when " +
			"its own object next changes. Precanned filters (`-IncludedRecipients` and `-Conditional*`) are not managed: an " +
			"imported precanned list shows the equivalent filter in `recipient_filter`, and applying writes it as a custom filter.",
		fields: []field{
			stringAttr("display_name", "DisplayName", "The name shown in Outlook. Defaults to `name`."),
			{
				name: "container", param: "Container", kind: stringField, forceNew: true,
				desc: "Name of the parent address list, for a list nested in another one. Empty for a top-level list. " +
					"Changing it creates a new list.",
			},
			filterAttr("recipient_filter", "RecipientFilter", "OPATH filter of the recipients in the list, for example "+
				"`Company -eq 'Contoso' -and RecipientType -eq 'UserMailbox'`. Exchange rewrites it with parentheses: the "+
				"configured text is kept as long as Exchange returns the text it wrote."),
		},
	},
	{
		typeName:    "global_address_list",
		noun:        "GlobalAddressList",
		description: "Manages a global address list (GAL), the main address list of an address book policy",
		notes: "Manage a custom GAL for each entity of the organization; the default GAL, `Default Global Address List`, " +
			"can be imported but not changed. Exchange Online cannot recalculate the members of a GAL: after a change of " +
			"`recipient_filter`, a recipient enters or leaves it when its own object next changes.",
		fields: []field{
			filterAttr("recipient_filter", "RecipientFilter", "OPATH filter of the recipients in the GAL, for example "+
				"`CustomAttribute1 -eq 'Contoso'`. Exchange rewrites it with parentheses: the configured text is kept as long "+
				"as Exchange returns the text it wrote."),
		},
	},
	{
		typeName:    "offline_address_book",
		noun:        "OfflineAddressBook",
		script:      "address_book.ps1",
		description: "Manages an offline address book (OAB), the address lists Outlook downloads for offline use",
		notes:       "Which offline address book is the default is not managed.",
		fields: []field{
			setAttr("address_lists", "AddressLists", "Names of the address lists in the offline address book, usually the "+
				"GAL of an address book policy. Required to create it."),
		},
	},
	{
		typeName: "address_book_policy",
		noun:     "AddressBookPolicy",
		script:   "address_book.ps1",
		description: "Manages an address book policy (ABP), which gives the mailboxes it is assigned to their own GAL, " +
			"address lists, offline address book and room list",
		notes: "All four references are required to create a policy; they are names of the corresponding resources. " +
			"Assigning the policy to mailboxes (`Set-Mailbox -AddressBookPolicy`) is not managed.",
		fields: []field{
			stringAttr("global_address_list", "GlobalAddressList", "Name of the global address list."),
			stringAttr("offline_address_book", "OfflineAddressBook", "Name of the offline address book."),
			stringAttr("room_list", "RoomList", "Name of the address list of the rooms users can book, an address list "+
				"whose filter selects room mailboxes."),
			setAttr("address_lists", "AddressLists", "Names of the address lists users see besides the GAL."),
		},
	},
	{
		typeName: "management_scope",
		noun:     "ManagementScope",
		script:   "management_scope.ps1",
		description: "Manages a management scope: the recipients a role assignment can change, selected by an " +
			"OPATH filter",
		notes: "Use it in `msexchangeonline_management_role_assignment` (`custom_recipient_write_scope`, or " +
			"`custom_resource_scope` for an application).",
		fields: []field{
			{
				name: "recipient_restriction_filter", param: "RecipientRestrictionFilter", property: "RecipientFilter",
				kind: filterField,
				desc: "OPATH filter of the recipients in the scope, for example `CustomAttribute1 -eq 'France'`. " +
					"Exchange may rewrite it with parentheses: the configured text is kept as long as Exchange returns " +
					"the text it wrote.",
			},
			stringAttr("recipient_root", "RecipientRoot", "Organizational unit the filter applies to. Empty for the "+
				"whole organization."),
			{
				name: "exclusive", param: "Exclusive", kind: boolField, forceNew: true,
				desc: "An exclusive scope: only the assignments that use it can change its recipients, even for " +
					"administrators with organization-wide roles. Changing it creates a new scope.",
			},
		},
	},
	{
		typeName: "role_group",
		noun:     "RoleGroup",
		script:   "role_group.ps1",
		description: "Manages a custom role group: an Exchange administration role group whose roles and scopes " +
			"come from `msexchangeonline_management_role_assignment`",
		notes: "Members and the managers of the membership (ManagedBy) are not managed: they come from your identity " +
			"management process, and a member added or removed outside Terraform never shows as a change. Built-in role groups (Organization Management, ...) " +
			"cannot be managed; read them with the data source.",
		fields: []field{
			stringAttr("display_name", "DisplayName", "Name shown in the Exchange admin center. Defaults to `name`."),
			stringAttr("description", "Description", "Description of the role group."),
		},
	},
	{
		typeName:  "irm_configuration",
		noun:      "IRMConfiguration",
		singleton: true,
		description: "Manages the Information Rights Management (IRM) configuration of the organization: Microsoft " +
			"Purview Message Encryption, and how Exchange decrypts protected messages",
		notes: "It needs Microsoft Purview Message Encryption (Microsoft 365 E3 or E5, Office 365 E3 or E5, or Azure " +
			"Information Protection Plan 1). Legacy AD RMS settings are not managed.",
		fields: []field{
			boolAttr("azure_rms_licensing_enabled", "AzureRMSLicensingEnabled", "Connect to Azure Rights Management: "+
				"turns Microsoft Purview Message Encryption on."),
			boolAttr("internal_licensing_enabled", "InternalLicensingEnabled", "Use IRM for messages sent inside the "+
				"organization."),
			boolAttr("simplified_client_access_enabled", "SimplifiedClientAccessEnabled", "Show the Protect button in "+
				"Outlook on the web."),
			boolAttr("simplified_client_access_encrypt_only_disabled", "SimplifiedClientAccessEncryptOnlyDisabled",
				"Hide the Encrypt-Only option in Outlook on the web."),
			boolAttr("simplified_client_access_do_not_forward_disabled", "SimplifiedClientAccessDoNotForwardDisabled",
				"Hide the Do Not Forward option in Outlook on the web."),
			enumAttr("transport_decryption_setting", "TransportDecryptionSetting", "Whether Exchange decrypts protected "+
				"messages in transport, for example to scan them.", "Disabled", "Optional", "Mandatory"),
			boolAttr("journal_report_decryption_enabled", "JournalReportDecryptionEnabled", "Add a decrypted copy of "+
				"protected messages to journal reports."),
			boolAttr("decrypt_attachment_for_encrypt_only", "DecryptAttachmentForEncryptOnly", "Give recipients "+
				"unrestricted rights on the attachments of Encrypt-Only messages."),
			boolAttr("enable_pdf_encryption", "EnablePdfEncryption", "Encrypt PDF attachments of encrypted messages."),
			boolAttr("enable_portal_tracking_logs", "EnablePortalTrackingLogs", "Log the activity of the encrypted "+
				"message portal (Advanced Message Encryption)."),
			boolAttr("search_enabled", "SearchEnabled", "Let users search encrypted messages in Outlook on the web."),
			boolAttr("ediscovery_super_user_enabled", "EDiscoverySuperUserEnabled", "Let eDiscovery searches read "+
				"protected messages."),
			boolAttr("reject_if_recipient_has_no_rights", "RejectIfRecipientHasNoRights", "Reject a protected message "+
				"sent to a recipient who has no rights on it."),
			boolAttr("automatic_service_update_enabled", "AutomaticServiceUpdateEnabled", "Let Microsoft add new "+
				"Azure Information Protection features automatically."),
		},
	},
	{
		typeName:     "ome_configuration",
		noun:         "OMEConfiguration",
		script:       "ome_configuration.ps1",
		listProperty: "Identity",
		description: "Manages a branding template of Microsoft Purview Message Encryption: the logo, colors and texts " +
			"of encrypted messages and of the encrypted message portal",
		notes: "`name = \"OME Configuration\"` adopts the default template: it is changed, never created, and destroying " +
			"the resource only removes it from the state. Other names are custom templates, which need Advanced Message " +
			"Encryption (E5); a transport rule applies them with `apply_rights_protection_customization_template`.",
		fields: []field{
			filterAttr("image", "Image", "Logo, as base64 (`filebase64(\"logo.png\")`): PNG, JPG, BMP or TIFF, ideally "+
				"under 40 KB and 170 x 70 pixels. `\"\"` removes the logo. Exchange re-encodes the image: the configured "+
				"value is kept as long as Exchange returns the image it stored at the last write."),
			stringAttr("background_color", "BackgroundColor", "Background color, as an HTML name or a hex code such as "+
				"`#ffffff`."),
			stringAttr("introduction_text", "IntroductionText", "Text after the sender's name and address."),
			stringAttr("email_text", "EmailText", "Text of the message the recipient receives."),
			stringAttr("read_button_text", "ReadButtonText", "Text of the Read the message button."),
			stringAttr("portal_text", "PortalText", "Text at the top of the encrypted message portal."),
			stringAttr("disclaimer_text", "DisclaimerText", "Disclaimer at the bottom of the message."),
			stringAttr("privacy_statement_url", "PrivacyStatementUrl", "Address of the privacy statement link."),
			boolAttr("otp_enabled", "OTPEnabled", "Let recipients sign in to the portal with a one-time passcode."),
			boolAttr("social_id_sign_in", "SocialIdSignIn", "Let recipients sign in with a Microsoft, Google or Yahoo account."),
			int64Attr("external_mail_expiry_in_days", "ExternalMailExpiryInDays", "Days external recipients can read "+
				"the message in the portal, from 1 to 730; `0` for no expiry. Custom templates only.",
				int64validator.Between(0, 730)),
		},
	},
}
