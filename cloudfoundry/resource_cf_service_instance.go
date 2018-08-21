package cloudfoundry

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform/helper/customdiff"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-cf/cloudfoundry/cfapi"
)

func resourceServiceInstance() *schema.Resource {

	return &schema.Resource{

		Create: resourceServiceInstanceCreate,
		Read:   resourceServiceInstanceRead,
		Update: resourceServiceInstanceUpdate,
		Delete: resourceServiceInstanceDelete,

		Importer: &schema.ResourceImporter{
			State: ImportStatePassthrough,
		},

		CustomizeDiff: customdiff.All(
			resourceServiceInstanceValidateDiff,
		),

		Schema: map[string]*schema.Schema{

			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"service_plan": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"space": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"json_params": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "",
				ValidateFunc: validation.ValidateJsonString,
			},
			"json_params_sensitive": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "",
				Sensitive:    true,
				ValidateFunc: validation.ValidateJsonString,
			},
			"tags": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceServiceInstanceValidateDiff(d *schema.ResourceDiff, meta interface{}) error {
	jsonParametersString, hasJson := d.GetOk("json_params")
	jsonParametersSensitiveString, hasJsonSensitive := d.GetOk("json_params_sensitive")
	if hasJson && hasJsonSensitive {
		var jsonParams map[string]interface{}
		if err := json.Unmarshal([]byte(jsonParametersString.(string)), &jsonParams); err != nil {
			return err
		}
		var jsonParamsSensitive map[string]interface{}
		if err := json.Unmarshal([]byte(jsonParametersSensitiveString.(string)), &jsonParamsSensitive); err != nil {
			return err
		}
		for k := range jsonParams {
			if _, hasKey := jsonParamsSensitive[k]; hasKey {
				return fmt.Errorf("json_params and json_params_sensitive contain overlapping top level keys (%s)", k)
			}
		}
	}
	return nil
}

func resourceServiceInstanceProcessJsonParams(d *schema.ResourceData) (map[string]interface{}, error) {
	var params map[string]interface{}

	if jsonParameters := d.Get("json_params").(string); len(jsonParameters) > 0 {
		if err := json.Unmarshal([]byte(jsonParameters), &params); err != nil {
			return params, err
		}
	}

	if jsonParameters := d.Get("json_params_sensitive").(string); len(jsonParameters) > 0 {
		var additionalParams map[string]interface{}
		if err := json.Unmarshal([]byte(jsonParameters), &params); err != nil {
			return params, err
		}
		for k, v := range additionalParams {
			params[k] = v
		}
	}

	return params, nil
}

func resourceServiceInstanceCreate(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	var (
		id     string
		tags   []string
		params map[string]interface{}
	)
	name := d.Get("name").(string)
	servicePlan := d.Get("service_plan").(string)
	space := d.Get("space").(string)

	params, err = resourceServiceInstanceProcessJsonParams(d)
	if err != nil {
		return err
	}

	for _, v := range d.Get("tags").([]interface{}) {
		tags = append(tags, v.(string))
	}

	sm := session.ServiceManager()

	if id, err = sm.CreateServiceInstance(name, servicePlan, space, params, tags); err != nil {
		return err
	}
	session.Log.DebugMessage("New Service Instance : %# v", id)

	// TODO deal with asynchronous responses

	d.SetId(id)

	return nil
}

func resourceServiceInstanceRead(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}
	session.Log.DebugMessage("Reading Service Instance : %s", d.Id())

	sm := session.ServiceManager()
	var serviceInstance cfapi.CCServiceInstance

	serviceInstance, err = sm.ReadServiceInstance(d.Id())
	if err != nil {
		return err
	}

	d.Set("name", serviceInstance.Name)
	d.Set("service_plan", serviceInstance.ServicePlanGUID)
	d.Set("space", serviceInstance.SpaceGUID)

	if serviceInstance.Tags != nil {
		tags := make([]interface{}, len(serviceInstance.Tags))
		for i, v := range serviceInstance.Tags {
			tags[i] = v
		}
		d.Set("tags", tags)
	} else {
		d.Set("tags", nil)
	}

	session.Log.DebugMessage("Read Service Instance : %# v", serviceInstance)

	return nil
}

func resourceServiceInstanceUpdate(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}
	sm := session.ServiceManager()

	session.Log.DebugMessage("begin resourceServiceInstanceUpdate")

	var (
		id, name string
		tags     []string
		params   map[string]interface{}
	)

	id = d.Id()
	name = d.Get("name").(string)
	servicePlan := d.Get("service_plan").(string)

	params, err = resourceServiceInstanceProcessJsonParams(d)
	if err != nil {
		return err
	}

	for _, v := range d.Get("tags").([]interface{}) {
		tags = append(tags, v.(string))
	}

	_, err = sm.UpdateServiceInstance(id, name, servicePlan, params, tags)
	return err
}

func resourceServiceInstanceDelete(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}
	session.Log.DebugMessage("begin resourceServiceInstanceDelete")

	sm := session.ServiceManager()

	err = sm.DeleteServiceInstance(d.Id())
	if err != nil {
		return err
	}

	session.Log.DebugMessage("Deleted Service Instance : %s", d.Id())

	return nil
}
