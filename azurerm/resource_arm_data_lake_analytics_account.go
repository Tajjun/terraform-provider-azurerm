package azurerm

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/services/datalake/analytics/mgmt/2016-11-01/account"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"

	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/response"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/suppress"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
)

func resourceArmDataLakeAnalyticsAccount() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmDateLakeAnalyticsAccountCreate,
		Read:   resourceArmDateLakeAnalyticsAccountRead,
		Update: resourceArmDateLakeAnalyticsAccountUpdate,
		Delete: resourceArmDateLakeAnalyticsAccountDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: azure.ValidateDataLakeAccountName(),
			},

			"location": azure.SchemaLocation(),

			"resource_group_name": azure.SchemaResourceGroupName(),

			"tier": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          string(account.Consumption),
				DiffSuppressFunc: suppress.CaseDifference,
				ValidateFunc: validation.StringInSlice([]string{
					string(account.Consumption),
					string(account.Commitment100000AUHours),
					string(account.Commitment10000AUHours),
					string(account.Commitment1000AUHours),
					string(account.Commitment100AUHours),
					string(account.Commitment500000AUHours),
					string(account.Commitment50000AUHours),
					string(account.Commitment5000AUHours),
					string(account.Commitment500AUHours),
				}, true),
			},

			"default_store_account_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: azure.ValidateDataLakeAccountName(),
			},

			"tags": tagsSchema(),
		},
	}
}

func resourceArmDateLakeAnalyticsAccountCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).datalake.AnalyticsAccountsClient
	ctx := meta.(*ArmClient).StopContext

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	if requireResourcesToBeImported {
		existing, err := client.Get(ctx, resourceGroup, name)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("Error checking for presence of existing Data Lake Analytics Account %q (Resource Group %q): %s", name, resourceGroup, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_data_lake_analytics_account", *existing.ID)
		}
	}

	location := azure.NormalizeLocation(d.Get("location").(string))
	storeAccountName := d.Get("default_store_account_name").(string)
	tier := d.Get("tier").(string)
	tags := d.Get("tags").(map[string]interface{})

	log.Printf("[INFO] preparing arguments for Azure ARM Date Lake Store creation %q (Resource Group %q)", name, resourceGroup)

	dateLakeAnalyticsAccount := account.CreateDataLakeAnalyticsAccountParameters{
		Location: &location,
		Tags:     expandTags(tags),
		CreateDataLakeAnalyticsAccountProperties: &account.CreateDataLakeAnalyticsAccountProperties{
			NewTier:                     account.TierType(tier),
			DefaultDataLakeStoreAccount: &storeAccountName,
			DataLakeStoreAccounts: &[]account.AddDataLakeStoreWithAccountParameters{
				{
					Name: &storeAccountName,
				},
			},
		},
	}

	future, err := client.Create(ctx, resourceGroup, name, dateLakeAnalyticsAccount)
	if err != nil {
		return fmt.Errorf("Error issuing create request for Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error creating Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	read, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		return fmt.Errorf("Error retrieving Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read Data Lake Analytics Account %s (resource group %s) ID", name, resourceGroup)
	}

	d.SetId(*read.ID)

	return resourceArmDateLakeAnalyticsAccountRead(d, meta)
}

func resourceArmDateLakeAnalyticsAccountUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).datalake.AnalyticsAccountsClient
	ctx := meta.(*ArmClient).StopContext

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)
	storeAccountName := d.Get("default_store_account_name").(string)
	newTier := d.Get("tier").(string)
	newTags := d.Get("tags").(map[string]interface{})

	props := &account.UpdateDataLakeAnalyticsAccountParameters{
		Tags: expandTags(newTags),
		UpdateDataLakeAnalyticsAccountProperties: &account.UpdateDataLakeAnalyticsAccountProperties{
			NewTier: account.TierType(newTier),
			DataLakeStoreAccounts: &[]account.UpdateDataLakeStoreWithAccountParameters{
				{
					Name: &storeAccountName,
				},
			},
		},
	}

	future, err := client.Update(ctx, resourceGroup, name, props)
	if err != nil {
		return fmt.Errorf("Error issuing update request for Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for the update of Data Lake Analytics Account %q (Resource Group %q) to commplete: %+v", name, resourceGroup, err)
	}

	return resourceArmDateLakeAnalyticsAccountRead(d, meta)
}

func resourceArmDateLakeAnalyticsAccountRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).datalake.AnalyticsAccountsClient
	ctx := meta.(*ArmClient).StopContext

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := id.ResourceGroup
	name := id.Path["accounts"]

	resp, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[WARN] DataLakeAnalyticsAccountAccount '%s' was not found (resource group '%s')", name, resourceGroup)
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error making Read request on Azure Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	d.Set("name", name)
	d.Set("resource_group_name", resourceGroup)
	if location := resp.Location; location != nil {
		d.Set("location", azure.NormalizeLocation(*location))
	}

	if properties := resp.DataLakeAnalyticsAccountProperties; properties != nil {
		d.Set("tier", string(properties.CurrentTier))
		d.Set("default_store_account_name", properties.DefaultDataLakeStoreAccount)
	}

	flattenAndSetTags(d, resp.Tags)

	return nil
}

func resourceArmDateLakeAnalyticsAccountDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).datalake.AnalyticsAccountsClient
	ctx := meta.(*ArmClient).StopContext

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resourceGroup := id.ResourceGroup
	name := id.Path["accounts"]
	future, err := client.Delete(ctx, resourceGroup, name)
	if err != nil {
		if response.WasNotFound(future.Response()) {
			return nil
		}
		return fmt.Errorf("Error issuing delete request for Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		if response.WasNotFound(future.Response()) {
			return nil
		}
		return fmt.Errorf("Error deleting Data Lake Analytics Account %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	return nil
}
