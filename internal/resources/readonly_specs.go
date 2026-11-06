package resources

// readOnlySpecs are Exchange objects that exist only as data sources: the
// Hybrid Configuration Wizard creates and updates them, the provider only
// reads them to refer to them from other resources. tools/tools_test.go reads
// this file for the Get- cmdlets of the role group.
var readOnlySpecs = []objectSpec{
	{
		typeName:    "onpremises_organization",
		noun:        "OnPremisesOrganization",
		description: "Reads the on-premises organization that the Hybrid Configuration Wizard creates in Exchange Online",
		fields: []field{
			setAttr("hybrid_domains", "HybridDomains", "Domains shared by the two organizations."),
			stringAttr("inbound_connector", "InboundConnector", "Inbound connector of the hybrid mail flow."),
			stringAttr("outbound_connector", "OutboundConnector", "Outbound connector of the hybrid mail flow."),
			stringAttr("organization_name", "OrganizationName", "Name of the on-premises Exchange organization."),
			stringAttr("organization_guid", "OrganizationGuid", "GUID of the on-premises Exchange organization."),
			stringAttr("organization_relationship", "OrganizationRelationship", "Organization relationship with the "+
				"on-premises organization."),
			stringAttr("comment", "Comment", "Comment written by the wizard."),
		},
	},
	{
		typeName: "intra_organization_connector",
		noun:     "IntraOrganizationConnector",
		description: "Reads an intra-organization connector, which the Hybrid Configuration Wizard creates for the " +
			"free/busy and MailTips of hybrid modern authentication",
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the connector is used."),
			stringAttr("discovery_endpoint", "DiscoveryEndpoint", "Autodiscover endpoint of the on-premises organization."),
			setAttr("target_address_domains", "TargetAddressDomains", "Domains of the on-premises organization."),
			stringAttr("target_sharing_epr", "TargetSharingEpr", "Exchange Web Services endpoint of the on-premises "+
				"organization, when Autodiscover cannot find it."),
		},
	},
}
