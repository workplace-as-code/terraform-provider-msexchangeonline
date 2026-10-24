package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// registration pairs a resource with the query that lists its objects.
type registration struct {
	newResource func() resource.Resource
	list        listQuery
}

// registrations returns every resource of the provider. A new resource is
// added here (generic resources are added to objectSpecs instead).
func registrations() []registration {
	regs := []registration{
		{NewTransportRuleResource, listQuery{noun: "TransportRule"}},
		{NewRemoteDomainResource, listQuery{noun: "RemoteDomain"}},
		{NewAcceptedDomainResource, listQuery{noun: "AcceptedDomain"}},
		{NewAntiPhishPolicyResource, listQuery{noun: "AntiPhishPolicy"}},
		{NewAntiPhishRuleResource, listQuery{noun: "AntiPhishRule"}},
		{NewMalwareFilterPolicyResource, listQuery{noun: "MalwareFilterPolicy"}},
		{NewMalwareFilterRuleResource, listQuery{noun: "MalwareFilterRule"}},
		{NewHostedContentFilterPolicyResource, listQuery{noun: "HostedContentFilterPolicy"}},
		{NewHostedContentFilterRuleResource, listQuery{noun: "HostedContentFilterRule"}},
		{NewSafeLinksPolicyResource, listQuery{noun: "SafeLinksPolicy"}},
		{NewSafeLinksRuleResource, listQuery{noun: "SafeLinksRule"}},
		{NewSafeAttachmentPolicyResource, listQuery{noun: "SafeAttachmentPolicy"}},
		{NewSafeAttachmentRuleResource, listQuery{noun: "SafeAttachmentRule"}},
		{NewOwaMailboxPolicyResource, listQuery{noun: "OwaMailboxPolicy"}},
		{NewOrganizationConfigResource, listQuery{id: "organization"}},
		{NewInboundConnectorResource, listQuery{noun: "InboundConnector"}},
		{NewOutboundConnectorResource, listQuery{noun: "OutboundConnector"}},
		{NewDkimSigningConfigResource, listQuery{noun: "DkimSigningConfig"}},
		{NewOutboundSpamFilterPolicyResource, listQuery{noun: "HostedOutboundSpamFilterPolicy"}},
		{NewQuarantinePolicyResource, listQuery{noun: "QuarantinePolicy", property: "Name"}},
		{NewEmailAddressPolicyResource, listQuery{noun: "EmailAddressPolicy"}},
		{NewMobileDeviceMailboxPolicyResource, listQuery{noun: "MobileDeviceMailboxPolicy"}},
		{NewRetentionPolicyResource, listQuery{noun: "RetentionPolicy"}},
		// list.ps1 lists the four lists and returns "<list_type>:<value>".
		{NewTenantAllowBlockListItemResource, listQuery{noun: "TenantAllowBlockListItems"}},
		{NewApplicationAccessPolicyResource, listQuery{noun: "ApplicationAccessPolicy"}},
	}
	for _, spec := range objectSpecs {
		regs = append(regs, registration{newObjectResource(spec), spec.listQuery()})
	}
	return regs
}

// Resources returns the resources registered in the provider, with their identity.
func Resources() []func() resource.Resource {
	regs := registrations()
	out := make([]func() resource.Resource, 0, len(regs))
	for _, reg := range regs {
		out = append(out, withIdentity(reg.newResource))
	}
	return out
}

// ListResources returns the list resource of every resource, for terraform query.
func ListResources() []func() list.ListResource {
	regs := registrations()
	out := make([]func() list.ListResource, 0, len(regs))
	for _, reg := range regs {
		out = append(out, newListResource(reg.newResource, reg.list))
	}
	return out
}
