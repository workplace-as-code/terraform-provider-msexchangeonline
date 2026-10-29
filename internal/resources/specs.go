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
				"Address that receives the journal reports that cannot be delivered, required before journal rules deliver anything. "+
					"It cannot be an Exchange Online mailbox. Once set, it can be changed but never cleared."),
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
		script:      "exo_rule.ps1",
		description: "Manages a journal rule, which sends a copy of the matching messages to an archive outside Exchange Online",
		notes: "`journal_email_address` is required to create a rule. Set `transport_config.journaling_report_ndr_to` first: " +
			"Exchange Online sends the journal reports it cannot deliver there.",
		fields: []field{
			stringAttr("journal_email_address", "JournalEmailAddress",
				"Address that receives the journal reports: an archiving system or service outside Exchange Online, never an Exchange Online mailbox."),
			stringAttr("recipient", "Recipient",
				"SMTP address of the mailbox, contact or group whose messages are journaled. `\"\"` journals the messages of every recipient."),
			enumAttr("scope", "Scope", "Messages the rule journals: all of them, internal ones or external ones.", "Global", "Internal", "External"),
			boolAttr("enabled", "Enabled", "Whether the rule is enabled."),
		},
	},
}
