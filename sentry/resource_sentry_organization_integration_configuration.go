package sentry

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/jianyuan/go-sentry/v2/sentry"
)

func resourceSentryOrganizationIntegrationConfiguration() *schema.Resource {
	return &schema.Resource{
		Description:   "Sentry Organization Integration Configuration resource.",
		ReadContext:   resourceSentryOrganizationIntegrationConfigurationRead,
		UpdateContext: resourceSentryOrganizationIntegrationConfigurationUpdate,
		DeleteContext: resourceSentryOrganizationIntegrationConfigurationDelete,

		Schema: map[string]*schema.Schema{
			"organization": {
				Description: "The slug of the organization.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"provider_key": {
				Description: "Specific integration provider to filter by such as `slack`. See [the list of supported providers](https://docs.sentry.io/product/integrations/).",
				Type:        schema.TypeString,
				Required:    true,
			},
			"name": {
				Description: "The name of the integration.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"config": {
				Description: "Integration configuration.",
				Type:        schema.TypeMap,
				Required:    true,
			},
			"internal_id": {
				Description: "The internal ID for this organization integration configuration.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceSentryOrganizationIntegrationConfigurationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	internal_id := d.Get("internal_id").(string)
	organization := d.Get("organization").(string)
	providerKey := d.Get("provider_key").(string)
	name := d.Get("name").(string)

	tflog.Debug(ctx, "Reading Sentry organization integration configuration", map[string]interface{}{
		"organization": organization,
		"providerKey":  providerKey,
		"name":         name,
	})

	var matchedIntegrations []*sentry.OrganizationIntegration
	params := &sentry.ListOrganizationIntegrationsParams{
		ListCursorParams: sentry.ListCursorParams{},
		ProviderKey:      providerKey,
	}

	for {
		integrations, apiResp, err := client.OrganizationIntegrations.List(ctx, organization, params)
		if err != nil {
			return diag.FromErr(fmt.Errorf("unable to read organization integration configurations: %w", err))
		}

		for _, integration := range integrations {
			if integration.Name == name {
				matchedIntegrations = append(matchedIntegrations, integration)
			}
		}

		if apiResp.Cursor == "" {
			break
		}
		params.ListCursorParams.Cursor = apiResp.Cursor
	}

	if len(matchedIntegrations) == 0 {
		return diag.Errorf("no matching organization integration configurations found with name %q", name)
	} else if len(matchedIntegrations) > 1 {
		return diag.Errorf("found multiple matching organization integration configurations with name %q", name)
	}

	d.SetId(buildThreePartID(organization, providerKey, internal_id))

	retErr := multierror.Append(
		d.Set("organization", organization),
		d.Set("provider_key", providerKey),
		d.Set("name", name),
		d.Set("config", matchedIntegrations[0].ConfigData),
	)

	return diag.FromErr(retErr.ErrorOrNil())
}

func resourceSentryOrganizationIntegrationConfigurationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	organization := d.Get("organization").(string)
	providerKey := d.Get("provider_key").(string)
	name := d.Get("name").(string)
	config := d.Get("config").(sentry.IntegrationConfigData)

	tflog.Debug(ctx, "Creating Sentry organization integration configuration", map[string]interface{}{
		"organization": organization,
		"providerKey":  providerKey,
		"name":         name,
		"config":       config,
	})

	_, err := client.OrganizationIntegrations.UpdateConfig(ctx, organization, d.Id(), &config)
	if err != nil {
		return diag.FromErr(fmt.Errorf("unable to update organization integration configuration: %w", err))
	}

	d.SetId(d.Id())

	return resourceSentryOrganizationIntegrationConfigurationRead(ctx, d, meta)
}

func resourceSentryOrganizationIntegrationConfigurationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	organization := d.Get("organization").(string)
	providerKey := d.Get("provider_key").(string)
	name := d.Get("name").(string)

	tflog.Debug(ctx, "Deleting Sentry organization integration configuration", map[string]interface{}{
		"organization": organization,
		"providerKey":  providerKey,
		"name":         name,
	})

	if _, err := client.OrganizationIntegrations.UpdateConfig(ctx, organization, d.Id(), &sentry.IntegrationConfigData{}); err != nil {
		return diag.FromErr(fmt.Errorf("unable to delete organization integration configuration: %w", err))
	}

	d.SetId("")

	return nil
}
