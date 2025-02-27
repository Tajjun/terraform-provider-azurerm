package azurerm

import (
	"fmt"
	"log"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/services/devtestlabs/mgmt/2016-05-15/dtl"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmDevTestVirtualNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmDevTestVirtualNetworkCreate,
		Read:   resourceArmDevTestVirtualNetworkRead,
		Update: resourceArmDevTestVirtualNetworkUpdate,
		Delete: resourceArmDevTestVirtualNetworkDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateDevTestVirtualNetworkName(),
			},

			"lab_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.DevTestLabName(),
			},

			// There's a bug in the Azure API where this is returned in lower-case
			// BUG: https://github.com/Azure/azure-rest-api-specs/issues/3964
			"resource_group_name": azure.SchemaResourceGroupNameDiffSuppress(),

			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"subnet": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				// whilst the API accepts multiple, in practice only one is usable
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"use_in_virtual_machine_creation": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      string(dtl.Allow),
							ValidateFunc: validate.DevTestVirtualNetworkUsagePermissionType(),
						},

						"use_public_ip_address": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      string(dtl.Allow),
							ValidateFunc: validate.DevTestVirtualNetworkUsagePermissionType(),
						},
					},
				},
			},

			"tags": tagsSchema(),

			"unique_identifier": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceArmDevTestVirtualNetworkCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).devTestLabs.VirtualNetworksClient
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[INFO] preparing arguments for DevTest Virtual Network creation")

	name := d.Get("name").(string)
	labName := d.Get("lab_name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	if requireResourcesToBeImported && d.IsNewResource() {
		existing, err := client.Get(ctx, resourceGroup, labName, name, "")
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("Error checking for presence of existing Dev Test Virtual Network %q (Lab %q / Resource Group %q): %s", name, labName, resourceGroup, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_dev_test_virtual_network", *existing.ID)
		}
	}

	description := d.Get("description").(string)
	tags := d.Get("tags").(map[string]interface{})

	subscriptionId := meta.(*ArmClient).subscriptionId
	subnetsRaw := d.Get("subnet").([]interface{})
	subnets := expandDevTestVirtualNetworkSubnets(subnetsRaw, subscriptionId, resourceGroup, name)

	parameters := dtl.VirtualNetwork{
		Tags: expandTags(tags),
		VirtualNetworkProperties: &dtl.VirtualNetworkProperties{
			Description:     utils.String(description),
			SubnetOverrides: subnets,
		},
	}

	future, err := client.CreateOrUpdate(ctx, resourceGroup, labName, name, parameters)
	if err != nil {
		return fmt.Errorf("Error creating DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for creation of DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	read, err := client.Get(ctx, resourceGroup, labName, name, "")
	if err != nil {
		return fmt.Errorf("Error retrieving DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	if read.ID == nil {
		return fmt.Errorf("Cannot read DevTest Virtual Network %q (Lab %q / Resource Group %q) ID", name, labName, resourceGroup)
	}

	d.SetId(*read.ID)

	return resourceArmDevTestVirtualNetworkUpdate(d, meta)
}

func resourceArmDevTestVirtualNetworkRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).devTestLabs.VirtualNetworksClient
	ctx := meta.(*ArmClient).StopContext

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	labName := id.Path["labs"]
	name := id.Path["virtualnetworks"]

	read, err := client.Get(ctx, resourceGroup, labName, name, "")
	if err != nil {
		if utils.ResponseWasNotFound(read.Response) {
			log.Printf("[DEBUG] DevTest Virtual Network %q was not found in Lab %q / Resource Group %q - removing from state!", name, labName, resourceGroup)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error making Read request on DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	d.Set("name", read.Name)
	d.Set("lab_name", labName)
	d.Set("resource_group_name", resourceGroup)

	if props := read.VirtualNetworkProperties; props != nil {
		d.Set("description", props.Description)

		flattenedSubnets := flattenDevTestVirtualNetworkSubnets(props.SubnetOverrides)
		if err := d.Set("subnet", flattenedSubnets); err != nil {
			return fmt.Errorf("Error setting `subnet`: %+v", err)
		}

		// Computed fields
		d.Set("unique_identifier", props.UniqueIdentifier)
	}

	flattenAndSetTags(d, read.Tags)

	return nil
}

func resourceArmDevTestVirtualNetworkUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).devTestLabs.VirtualNetworksClient
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[INFO] preparing arguments for DevTest Virtual Network creation")

	name := d.Get("name").(string)
	labName := d.Get("lab_name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	description := d.Get("description").(string)
	tags := d.Get("tags").(map[string]interface{})

	subscriptionId := meta.(*ArmClient).subscriptionId
	subnetsRaw := d.Get("subnet").([]interface{})
	subnets := expandDevTestVirtualNetworkSubnets(subnetsRaw, subscriptionId, resourceGroup, name)

	parameters := dtl.VirtualNetwork{
		Tags: expandTags(tags),
		VirtualNetworkProperties: &dtl.VirtualNetworkProperties{
			Description:     utils.String(description),
			SubnetOverrides: subnets,
		},
	}

	future, err := client.CreateOrUpdate(ctx, resourceGroup, labName, name, parameters)
	if err != nil {
		return fmt.Errorf("Error updating DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for update of DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	read, err := client.Get(ctx, resourceGroup, labName, name, "")
	if err != nil {
		return fmt.Errorf("Error retrieving DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	if read.ID == nil {
		return fmt.Errorf("Cannot read DevTest Virtual Network %q (Lab %q / Resource Group %q) ID", name, labName, resourceGroup)
	}

	d.SetId(*read.ID)

	return resourceArmDevTestVirtualNetworkRead(d, meta)
}

func resourceArmDevTestVirtualNetworkDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).devTestLabs.VirtualNetworksClient
	ctx := meta.(*ArmClient).StopContext

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	labName := id.Path["labs"]
	name := id.Path["virtualnetworks"]

	read, err := client.Get(ctx, resourceGroup, labName, name, "")
	if err != nil {
		if utils.ResponseWasNotFound(read.Response) {
			// deleted outside of TF
			log.Printf("[DEBUG] DevTest Virtual Network %q was not found in Lab %q / Resource Group %q - assuming removed!", name, labName, resourceGroup)
			return nil
		}

		return fmt.Errorf("Error retrieving DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	future, err := client.Delete(ctx, resourceGroup, labName, name)
	if err != nil {
		return fmt.Errorf("Error deleting DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for the deletion of DevTest Virtual Network %q (Lab %q / Resource Group %q): %+v", name, labName, resourceGroup, err)
	}

	return err
}

func validateDevTestVirtualNetworkName() schema.SchemaValidateFunc {
	return validation.StringMatch(
		regexp.MustCompile("^[A-Za-z0-9_-]+$"),
		"Virtual Network Name can only include alphanumeric characters, underscores, hyphens.")
}

func expandDevTestVirtualNetworkSubnets(input []interface{}, subscriptionId, resourceGroupName, virtualNetworkName string) *[]dtl.SubnetOverride {
	results := make([]dtl.SubnetOverride, 0)
	// default found from the Portal
	name := fmt.Sprintf("%sSubnet", virtualNetworkName)
	idFmt := "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s"
	subnetId := fmt.Sprintf(idFmt, subscriptionId, resourceGroupName, virtualNetworkName, name)
	if len(input) == 0 {
		result := dtl.SubnetOverride{
			ResourceID:                   utils.String(subnetId),
			LabSubnetName:                utils.String(name),
			UsePublicIPAddressPermission: dtl.Allow,
			UseInVMCreationPermission:    dtl.Allow,
		}
		results = append(results, result)
		return &results
	}

	for _, val := range input {
		v := val.(map[string]interface{})
		usePublicIPAddress := v["use_public_ip_address"].(string)
		useInVirtualMachineCreation := v["use_in_virtual_machine_creation"].(string)

		subnet := dtl.SubnetOverride{
			ResourceID:                   utils.String(subnetId),
			LabSubnetName:                utils.String(name),
			UsePublicIPAddressPermission: dtl.UsagePermissionType(usePublicIPAddress),
			UseInVMCreationPermission:    dtl.UsagePermissionType(useInVirtualMachineCreation),
		}
		results = append(results, subnet)
	}

	return &results
}

func flattenDevTestVirtualNetworkSubnets(input *[]dtl.SubnetOverride) []interface{} {
	outputs := make([]interface{}, 0)
	if input == nil {
		return outputs
	}

	for _, v := range *input {
		output := make(map[string]interface{})
		if v.LabSubnetName != nil {
			output["name"] = *v.LabSubnetName
		}
		output["use_public_ip_address"] = string(v.UsePublicIPAddressPermission)
		output["use_in_virtual_machine_creation"] = string(v.UseInVMCreationPermission)

		outputs = append(outputs, output)
	}

	return outputs
}
