package azurerm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func dataSourceArmAvailabilitySet() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceArmAvailabilitySetRead,
		Schema: map[string]*schema.Schema{
			"resource_group_name": azure.SchemaResourceGroupNameForDataSource(),

			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validate.NoEmptyStrings,
			},

			"location": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"platform_update_domain_count": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"platform_fault_domain_count": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"managed": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"tags": tagsForDataSourceSchema(),
		},
	}
}

func dataSourceArmAvailabilitySetRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).compute.AvailabilitySetsClient
	ctx := meta.(*ArmClient).StopContext

	resGroup := d.Get("resource_group_name").(string)
	name := d.Get("name").(string)

	resp, err := client.Get(ctx, resGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			return fmt.Errorf("Error: Availability Set %q (Resource Group %q) was not found", name, resGroup)
		}

		return fmt.Errorf("Error making Read request on Availability Set %q (Resource Group %q): %+v", name, resGroup, err)
	}

	d.SetId(*resp.ID)
	if location := resp.Location; location != nil {
		d.Set("location", azure.NormalizeLocation(*location))
	}
	if resp.Sku != nil && resp.Sku.Name != nil {
		d.Set("managed", strings.EqualFold(*resp.Sku.Name, "Aligned"))
	}
	if props := resp.AvailabilitySetProperties; props != nil {
		if v := props.PlatformUpdateDomainCount; v != nil {
			d.Set("platform_update_domain_count", strconv.Itoa(int(*v)))
		}
		if v := props.PlatformFaultDomainCount; v != nil {
			d.Set("platform_fault_domain_count", strconv.Itoa(int(*v)))
		}
	}
	flattenAndSetTags(d, resp.Tags)

	return nil
}
