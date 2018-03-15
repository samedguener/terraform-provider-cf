package cloudfoundry

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/terraform-providers/terraform-provider-cf/cloudfoundry/cfapi"
)

func resourceServiceInstance() *schema.Resource {

	return &schema.Resource{

		Create: resourceServiceInstanceCreate,
		Read:   resourceServiceInstanceRead,
		Update: resourceServiceInstanceUpdate,
		Delete: resourceServiceInstanceDelete,
		Importer: &schema.ResourceImporter{
			State: resourceServiceInstanceImport,
		},

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
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"tags": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  DefaultAppTimeout,
			},
			"recursive_delete": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"async": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
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
	jsonParameters := d.Get("json_params").(string)
	async := d.Get("async").(bool)

	for _, v := range d.Get("tags").([]interface{}) {
		tags = append(tags, v.(string))
	}

	if len(jsonParameters) > 0 {
		if err = json.Unmarshal([]byte(jsonParameters), &params); err != nil {
			return
		}
	}

	sm := session.ServiceManager()

	if !async {
		if id, err = sm.CreateServiceInstance(name, servicePlan, space, params, tags); err != nil {
			return
		}
	} else {
		if id, err = sm.CreateServiceInstanceAsync(name, servicePlan, space, params, tags); err != nil {
			return
		}
	}

	// Check whetever service_instance exists and is in state 'succeeded'
	timeout := time.Second * time.Duration(d.Get("timeout").(int))
	if err = sm.WaitServiceInstanceTo("create", id, timeout); err != nil {
		return
	}

	session.Log.DebugMessage("New Service Instance : %# v", id)

	// TODO deal with asynchronous responses

	d.SetId(id)

	return
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
		return
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

	return
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
	jsonParameters := d.Get("json_params").(string)
	async := d.Get("async").(bool)

	if len(jsonParameters) > 0 {
		if err = json.Unmarshal([]byte(jsonParameters), &params); err != nil {
			return
		}
	}

	for _, v := range d.Get("tags").([]interface{}) {
		tags = append(tags, v.(string))
	}

	if !async {
		if _, err = sm.UpdateServiceInstance(id, name, servicePlan, params, tags); err != nil {
			return
		}
	} else {
		if _, err = sm.UpdateServiceInstanceAsync(id, name, servicePlan, params, tags); err != nil {
			return
		}
	}
	if err != nil {
		return
	}

	// Check whetever service_instance exists and is in state 'succeeded'
	timeout := time.Second * time.Duration(d.Get("timeout").(int))
	if err = sm.WaitServiceInstanceTo("update", id, timeout); err != nil {
		return
	}

	return
}

func resourceServiceInstanceDelete(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}
	session.Log.DebugMessage("begin resourceServiceInstanceDelete")

	sm := session.ServiceManager()
	recursiveDelete := d.Get("recursive_delete").(bool)
	async := d.Get("async").(bool)

	if !async {
		err = sm.DeleteServiceInstance(d.Id(), recursiveDelete)
		if err != nil {
			return
		}
	} else {
		err = sm.DeleteServiceInstanceAsync(d.Id(), recursiveDelete)
		if err != nil {
			return
		}
	}

	session.Log.DebugMessage("Deleted Service Instance : %s", d.Id())

	return
}

func resourceServiceInstanceImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	session := meta.(*cfapi.Session)

	if session == nil {
		return nil, fmt.Errorf("client is nil")
	}

	sm := session.ServiceManager()

	serviceinstance, err := sm.ReadServiceInstance(d.Id())

	if err != nil {
		return nil, err
	}

	d.Set("name", serviceinstance.Name)
	d.Set("service_plan", serviceinstance.ServicePlanGUID)
	d.Set("space", serviceinstance.SpaceGUID)
	d.Set("tags", serviceinstance.Tags)

	// json_param can't be retrieved from CF, please inject manually if necessary
	d.Set("json_param", "")

	return []*schema.ResourceData{d}, nil
}
