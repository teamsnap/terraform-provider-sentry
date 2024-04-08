package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jianyuan/go-sentry/v2/sentry"
)

type organizationIntegrationConfigurationResource struct {
	client *sentry.Client
}

type organizationIntegrationConfigurationResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Organization types.String `tfsdk:"organization"`
	ProviderKey  types.String `tfsdk:"provider_key"`
	IsFragment   types.Bool   `tfsdk:"is_fragment"`
	Name         types.String `tfsdk:"name"`
	ConfigData   types.String `tfsdk:"configData"`
}

func NewOrganizationIntegrationConfigurationResource() *organizationIntegrationConfigurationResource {
	return &organizationIntegrationConfigurationResource{}
}

func (r *organizationIntegrationConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*sentry.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *sentry.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *organizationIntegrationConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_integration_configuration"
}

func (r *organizationIntegrationConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Organization integration configuration",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Description: "The slug of the organization.",
				Required:    true,
			},
			"provider_key": schema.StringAttribute{
				Description: "Specific integration provider to filter by such as `slack`. See [the list of supported providers](https://docs.sentry.io/product/integrations/).",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the integration.",
				Required:    true,
			},
			"is_fragment": schema.BoolAttribute{
				Description: "Whether the integration configuration is a fragment. Terraform will attempt to merge the provided configuration with the existing configuration if set to true and manage the partial configuration separately in state.",
				Optional:    true,
			},
			"config": schema.MapAttribute{
				ElementType: types.MapType{
					ElemType: types.StringType,
				},
				Description: "The organization integration configuration in JSON format.",
				Required:    true,
			},
		},
	}
}

func (r *organizationIntegrationConfigurationResourceModel) Fill(organizationSlug string, configData []byte, d sentry.OrganizationIntegration) error {
	r.Id = types.StringValue(d.ID)
	r.Organization = types.StringValue(organizationSlug)
	r.ProviderKey = types.StringValue(d.Provider.Key)
	r.Name = types.StringValue(d.Name)
	r.ConfigData = types.StringValue(string(configData))

	return nil
}

func (r *organizationIntegrationConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data organizationIntegrationConfigurationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var matchedIntegrations []*sentry.OrganizationIntegration
	params := &sentry.ListOrganizationIntegrationsParams{
		ListCursorParams: sentry.ListCursorParams{},
		ProviderKey:      data.ProviderKey.ValueString(),
	}

	for {
		integrations, apiResp, err := r.client.OrganizationIntegrations.List(ctx, data.Organization.ValueString(), params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read organization integrations, got error: %s", err))
			return
		}

		for _, integration := range integrations {
			if integration.Name == data.Name.ValueString() {
				matchedIntegrations = append(matchedIntegrations, integration)
			}
		}

		if apiResp.Cursor == "" {
			break
		}
		params.ListCursorParams.Cursor = apiResp.Cursor
	}

	if len(matchedIntegrations) == 0 {
		resp.Diagnostics.AddError("Not Found", "No matching organization integrations found")
		return
	} else if len(matchedIntegrations) > 1 {
		resp.Diagnostics.AddError("Not Unique", "More than one matching organization integration found")
		return
	}

	configData, err := json.Marshal(matchedIntegrations[0].ConfigData)
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Failed to convert ConfigData to JSON: %s", err.Error()))
		return
	}

	if err := data.Fill(data.Organization.ValueString(), configData, *matchedIntegrations[0]); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error filling organization integration: %s", err.Error()))
		return
	}

}

func (r *organizationIntegrationConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationIntegrationConfigurationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configData := map[string]interface{}{}
	if err := json.Unmarshal([]byte(data.ConfigData.ValueString()), &configData); err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Failed to convert ConfigData to JSON: %s", err.Error()))
		return
	}

	_, err := r.client.OrganizationIntegrations.UpdateConfig(ctx, data.Organization.ValueString(), data.Id.ValueString(), configData)

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create organization integration, got error: %s", err))
		return
	}

	resp.State.Set(ctx, &data)
}
