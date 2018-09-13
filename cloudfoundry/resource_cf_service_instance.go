package cloudfoundry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/hashicorp/terraform/helper/resource"
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
			State: resourceServiceInstanceImport,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Update: schema.DefaultTimeout(15 * time.Minute),
			Delete: schema.DefaultTimeout(15 * time.Minute),
		},

		CustomizeDiff: customdiff.All(
			resourceServiceInstanceValidateDiff,
		),

		SchemaVersion: 1,
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
			"service_plan_concurrency": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Allows for the concurrency of changes to service instances, sharing a particular service_plan, to be restricted.",
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

	if sem := limitConcurrency(d); sem != nil {
		defer (*sem).Release(1)
	}

	if id, err = sm.CreateServiceInstance(name, servicePlan, space, params, tags); err != nil {
		return err
	}
	stateConf := &resource.StateChangeConf{
		Pending:      resourceServiceInstancePendingStates,
		Target:       resourceServiceInstanceSucceesStates,
		Refresh:      resourceServiceInstanceStateFunc(id, "create", meta),
		Timeout:      d.Timeout(schema.TimeoutCreate),
		PollInterval: 30 * time.Second,
		Delay:        5 * time.Second,
	}

	// Wait, catching any errors
	if _, err = stateConf.WaitForState(); err != nil {
		return err
	}

	session.Log.DebugMessage("New Service Instance : %# v", id)

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
		if strings.Contains(err.Error(), "status code: 404") {
			d.SetId("")
			err = nil
		}
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

	// Enable partial state mode
	// We need to explicitly set state updates ourselves or
	// tell terraform when a state change is applied and thus okay to persist
	// In particular this is necessary for params since we cannot query CF for
	// the current value of this field
	d.Partial(true)

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

	if sem := limitConcurrency(d); sem != nil {
		defer (*sem).Release(1)
	}

	if _, err = sm.UpdateServiceInstance(id, name, servicePlan, params, tags); err != nil {
		return err
	}

	stateConf := &resource.StateChangeConf{
		Pending:      resourceServiceInstancePendingStates,
		Target:       resourceServiceInstanceSucceesStates,
		Refresh:      resourceServiceInstanceStateFunc(id, "update", meta),
		Timeout:      d.Timeout(schema.TimeoutUpdate),
		PollInterval: 30 * time.Second,
		Delay:        5 * time.Second,
	}
	// Wait, catching any errors
	if _, err = stateConf.WaitForState(); err != nil {
		return err
	}

	// We succeeded, disable partial mode
	d.Partial(false)
	return nil
}

func resourceServiceInstanceDelete(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	id := d.Id()

	if session == nil {
		return fmt.Errorf("client is nil")
	}
	session.Log.DebugMessage("begin resourceServiceInstanceDelete")

	sm := session.ServiceManager()

	if sem := limitConcurrency(d); sem != nil {
		defer (*sem).Release(1)
	}

	if err = sm.DeleteServiceInstance(id); err != nil {
		return err
	}
	stateConf := &resource.StateChangeConf{
		Pending:      resourceServiceInstancePendingStates,
		Target:       resourceServiceInstanceSucceesStates,
		Refresh:      resourceServiceInstanceStateFunc(id, "delete", meta),
		Timeout:      d.Timeout(schema.TimeoutDelete),
		PollInterval: 30 * time.Second,
		Delay:        5 * time.Second,
	}
	// Wait, catching any errors
	if _, err = stateConf.WaitForState(); err != nil {
		return err
	}

	session.Log.DebugMessage("Deleted Service Instance : %s", d.Id())

	return nil
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

	return ImportStatePassthrough(d, meta)
}

func resourceServiceInstanceStateFunc(serviceInstanceID string, operationType string, meta interface{}) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		session := meta.(*cfapi.Session)
		sm := session.ServiceManager()
		var err error
		var serviceInstance cfapi.CCServiceInstance
		if serviceInstance, err = sm.ReadServiceInstance(serviceInstanceID); err != nil {
			// if the service instance is gone the error message should contain error code 60004 ("ServiceInstanceNotFound")
			// which is the correct behavour if the service instance has been deleted
			// e.g. CLI output: cf_service_instance.redis: Server error, status code: 404, error code: 60004, message: The service instance could not be found: babababa-d977-4e9c-9bd0-4903d146d822
			if strings.Contains(err.Error(), "error code: 60004") && operationType == "delete" {
				return serviceInstance, "succeeded", nil
			} else {
				session.Log.DebugMessage("Error on retrieving the serviceInstance %s", serviceInstanceID)
				return nil, "", err
			}
			return nil, "", err
		}

		if serviceInstance.LastOperation["type"] == operationType {
			state := serviceInstance.LastOperation["state"]
			switch state {
			case "succeeded":
				return serviceInstance, "succeeded", nil
			case "failed":
				session.Log.DebugMessage("service instance with guid=%s async provisioning has failed", serviceInstanceID)
				return nil, "", err
			}
		}

		return serviceInstance, "in progress", nil
	}
}

var resourceServiceInstancePendingStates = []string{
	"in progress",
}

var resourceServiceInstanceSucceesStates = []string{
	"succeeded",
}

// #######################
// # Concurrency Limiter #
// #######################
// Updates to some types of services in Cloud Foundry (generally badly behaved service brokers)
// cannot be done in parallel or need to be done with limited concurrency.  This is a hack around
// the lack of a terraform provided method to limit the level of concurrency around a particular
// type of resource.  The idea here is that for all of the cf_service_instance resources
// which share a service_plan ID and set the service_plan_concurrency to a value greater than
// zero, then this code will cause all creates/updates/deletes of those service plan instances
// to be throttled to the defined concurrency limit.
//
// Limitations
// - The concurrency defined by the first resource to use a given service_plan ID wins
// - cf_service_instance resources of the same service plan which do not define service_plan_concurrency
//   will not take part in the limitation on concurrency

var concurrencySemaphore = make(map[string]*semaphore.Weighted)
var concurrencySemaphoreMutex = &sync.Mutex{}

func limitConcurrency(d *schema.ResourceData) *semaphore.Weighted {
	if d.Get("service_plan_concurrency").(int) <= 0 {
		// if no limit, then just skip
		return nil
	}

	concurrencySemaphoreMutex.Lock()
	if _, ok := concurrencySemaphore[d.Get("service_plan").(string)]; !ok {
		concurrencySemaphore[d.Get("service_plan").(string)] = semaphore.NewWeighted(int64(d.Get("service_plan_concurrency").(int)))
	}
	sem := concurrencySemaphore[d.Get("service_plan").(string)]
	concurrencySemaphoreMutex.Unlock()

	sem.Acquire(context.TODO(), 1)
	return sem
}
