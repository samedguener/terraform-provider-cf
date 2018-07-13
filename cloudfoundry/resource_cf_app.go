package cloudfoundry

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/cf/terminal"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-cf/cloudfoundry/cfapi"
)

// DefaultAppTimeout - Timeout (in seconds) when pushing apps to CF
const DefaultAppTimeout = 60

func resourceApp() *schema.Resource {

	return &schema.Resource{

		Create: resourceAppCreate,
		Read:   resourceAppRead,
		Update: resourceAppUpdate,
		Delete: resourceAppDelete,

		Importer: &schema.ResourceImporter{
			State: resourceAppImport,
		},

		SchemaVersion: 4,
		Schema: map[string]*schema.Schema{

			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"space": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"ports": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
				Set:      resourceIntegerSet,
			},
			"instances": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  1,
			},
			"memory": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"disk_quota": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"stack": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			"buildpack": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"command": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"enable_ssh": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  DefaultAppTimeout,
			},
			"stopped": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"url": &schema.Schema{
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"git", "github_release"},
				ValidateFunc:  validation.NoZeroValues,
			},
			"git": &schema.Schema{
				Type:          schema.TypeList,
				Optional:      true,
				MaxItems:      1,
				ConflictsWith: []string{"url", "github_release"},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"url": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"branch": &schema.Schema{
							Type:          schema.TypeString,
							Optional:      true,
							Default:       "master",
							ConflictsWith: []string{"git.tag"},
						},
						"tag": &schema.Schema{
							Type:          schema.TypeString,
							Optional:      true,
							ConflictsWith: []string{"git.branch"},
						},
						"user": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"password": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"key": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"github_release": &schema.Schema{
				Type:          schema.TypeList,
				Optional:      true,
				MaxItems:      1,
				ConflictsWith: []string{"url", "git"},
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"owner": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"repo": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"token": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"version": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"filename": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"add_content": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"source": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"destination": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"service_binding": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"service_instance": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"params": &schema.Schema{
							Type:     schema.TypeMap,
							Optional: true,
						},
						"credentials": &schema.Schema{
							Type:     schema.TypeMap,
							Computed: true,
						},
						"binding_id": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"route": &schema.Schema{
				Type:          schema.TypeList,
				Optional:      true,
				MaxItems:      1,
				ConflictsWith: []string{"routes", "blue_green"},
				Deprecated:    "Use the new 'routes' block for live routes and see the blue_green section for staging routes.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"default_route": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"default_route_mapping_id": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"stage_route": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Removed:  "Support for the non-default route has been removed.",
						},
						"stage_route_mapping_id": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"live_route": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Removed:  "Support for the non-default route has been removed.",
						},
						"live_route_mapping_id": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"validation_script": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Removed:  "Use blue_green.validation_script instead.",
						},
					},
				},
			},
			"routes": &schema.Schema{
				Type:          schema.TypeSet,
				Optional:      true,
				MinItems:      1,
				ConflictsWith: []string{"route"},
				Set:           hashRouteMappingSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"route": &schema.Schema{
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.NoZeroValues,
						},
						"port": &schema.Schema{
							Type:         schema.TypeInt,
							Optional:     true,
							Computed:     true,
							Deprecated:   "Not yet implemented!",
							ValidateFunc: validation.IntBetween(1, 65535),
						},
						"mapping_id": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"environment": &schema.Schema{
				Type:      schema.TypeMap,
				Optional:  true,
				Computed:  true,
				Sensitive: true,
			},
			"health_check_http_endpoint": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"health_check_type": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validateAppHealthCheckType,
			},
			"health_check_timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"disable_blue_green_deployment": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Removed:  "See new blue_green section instead to enable blue/green type updates.",
			},
			"blue_green": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enable": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"validation_script": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"shutdown_wait": &schema.Schema{
							Type:         schema.TypeInt,
							Description:  "Period (in minutes) to wait before shutting down the venerable application.",
							Optional:     true,
							Default:      0,
							ValidateFunc: validation.IntBetween(0, 15),
						},
						"forget_venerable": &schema.Schema{
							Type:        schema.TypeBool,
							Description: "Simply forget about the venerable version of the application instead of shutting it down.",
							Optional:    true,
							Default:     false,
						},
						"staging_route": &schema.Schema{
							Type:     schema.TypeSet,
							Optional: true,
							MinItems: 1,
							Set:      hashRouteMappingSet,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"route": &schema.Schema{
										Type:         schema.TypeString,
										Required:     true,
										ValidateFunc: validation.NoZeroValues,
									},
									"port": &schema.Schema{
										Type:         schema.TypeInt,
										Optional:     true,
										Computed:     true,
										Deprecated:   "Not yet implemented!",
										ValidateFunc: validation.IntBetween(1, 65535),
									},
									"mapping_id": &schema.Schema{
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
					},
				},
			},
			"deposed": {
				// This is not flagged as computed so that Terraform will always flag deposed resources as a change and allow us to attempt to clean them up
				Type:         schema.TypeMap,
				Optional:     true,
				Description:  "Do not use this, this field is meant for internal use only. (It is not flagged as Computed for technical reasons.)",
				ValidateFunc: validateAppDeposedMapEmpty,
			},
		},
	}
}

// func serviceBindingHash(d interface{}) int {
// 	return hashcode.String(d.(map[string]interface{})["service_instance"].(string))
// }

func validateAppHealthCheckType(v interface{}, k string) (ws []string, errs []error) {
	value := v.(string)
	if value != "port" && value != "process" && value != "http" && value != "none" {
		errs = append(errs, fmt.Errorf("%q must be one of 'port', 'process', 'http' or 'none'", k))
	}
	return ws, errs
}

func validateAppDeposedMapEmpty(v interface{}, k string) (ws []string, errs []error) {
	if len(v.(map[string]interface{})) != 0 {
		errs = append(errs, fmt.Errorf("%q must not be set by the user", k))
	}
	return ws, errs
}

type cfAppConfig struct {
	app             cfapi.CCApp
	routeConfig     map[string]interface{}
	routesConfig    []interface{}
	serviceBindings []map[string]interface{}
}

func resourceAppCreate(d *schema.ResourceData, meta interface{}) error {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	var app cfapi.CCApp
	app.Name = d.Get("name").(string)
	app.SpaceGUID = d.Get("space").(string)
	if v, ok := d.GetOk("ports"); ok {
		p := []int{}
		for _, vv := range v.(*schema.Set).List() {
			p = append(p, vv.(int))
		}
		app.Ports = &p
	}
	if v, ok := d.GetOk("instances"); ok {
		vv := v.(int)
		app.Instances = &vv
	}
	if v, ok := d.GetOk("memory"); ok {
		vv := v.(int)
		app.Memory = &vv
	}
	if v, ok := d.GetOk("disk_quota"); ok {
		vv := v.(int)
		app.DiskQuota = &vv
	}
	if v, ok := d.GetOk("stack"); ok {
		vv := v.(string)
		app.StackGUID = &vv
	}
	if v, ok := d.GetOk("buildpack"); ok {
		vv := v.(string)
		app.Buildpack = &vv
	}
	if v, ok := d.GetOk("command"); ok {
		vv := v.(string)
		app.Command = &vv
	}
	if v, ok := d.GetOk("enable_ssh"); ok {
		vv := v.(bool)
		app.EnableSSH = &vv
	}
	if v, ok := d.GetOk("health_check_http_endpoint"); ok {
		vv := v.(string)
		app.HealthCheckHTTPEndpoint = &vv
	}
	if v, ok := d.GetOk("health_check_type"); ok {
		vv := v.(string)
		app.HealthCheckType = &vv
	}
	if v, ok := d.GetOk("health_check_timeout"); ok {
		vv := v.(int)
		app.HealthCheckTimeout = &vv
	}
	if v, ok := d.GetOk("environment"); ok {
		vv := v.(map[string]interface{})
		app.Environment = &vv
	}

	appConfig := cfAppConfig{
		app: app,
	}

	if err := resourceAppCreateCfApp(d, meta, &appConfig); err != nil {
		return err
	}

	d.SetId(appConfig.app.ID)
	setAppArguments(appConfig.app, d)
	if len(appConfig.serviceBindings) > 0 {
		d.Set("service_binding", appConfig.serviceBindings)
	}
	if len(appConfig.routeConfig) > 0 {
		d.Set("route", []map[string]interface{}{appConfig.routeConfig})
	}
	if len(appConfig.routesConfig) > 0 {
		if err := d.Set("routes", schema.NewSet(hashRouteMappingSet, appConfig.routesConfig)); err != nil {
			return err
		}
	}

	return nil
}

func resourceAppCreateCfApp(d *schema.ResourceData, meta interface{}, appConfig *cfAppConfig) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	am := session.AppManager()
	rm := session.RouteManager()

	app := appConfig.app
	var (
		v interface{}

		defaultRoute string

		serviceBindings    []map[string]interface{}
		hasServiceBindings bool

		routeConfig map[string]interface{}
	)

	// Download application binary / source asynchronously
	appPathChan, errChan := prepareApp(app, d, session.Log)

	if v, hasRouteConfig := d.GetOk("route"); hasRouteConfig {

		routeConfig = v.([]interface{})[0].(map[string]interface{})
		if defaultRoute, err = validateRouteLegacy(routeConfig, "default_route", d.Id(), rm); err != nil {
			return err
		}
	}

	// Create application
	if app, err = am.CreateApp(app); err != nil {
		return err
	}
	// Delete application if an error occurs
	defer func() error {
		e := &err
		if *e != nil {
			return am.DeleteApp(app.ID, true)
		}
		return nil
	}()

	var addContent []map[string]interface{}
	if v, ok := d.GetOk("add_content"); ok {
		addContent = getListOfStructs(v)
	}
	// Upload application binary / source asynchronously once download has completed
	upload := make(chan error)
	go func() {
		appPath := <-appPathChan
		err := <-errChan
		if err != nil {
			upload <- err
			return
		}
		err = am.UploadApp(app, appPath, addContent)
		upload <- err
	}()

	// Bind services
	if v, hasServiceBindings = d.GetOk("service_binding"); hasServiceBindings {
		if serviceBindings, err = addServiceBindings(app.ID, getListOfStructs(v), am, session.Log); err != nil {
			return err
		}
	}

	if _, hasRouteConfig := d.GetOk("route"); hasRouteConfig {
		// old style route block (won't be compatible with blue/green anyways)
		if len(defaultRoute) > 0 {
			// Bind default route
			var mappingID string
			if mappingID, err = rm.CreateRouteMapping(defaultRoute, app.ID, nil); err != nil {
				return err
			}
			routeConfig["default_route_mapping_id"] = mappingID
			appConfig.routeConfig = routeConfig
			session.Log.DebugMessage("Created routes: %# v", d.Get("route"))
		}
	} else if v, hasRouteConfig := d.GetOk("routes"); hasRouteConfig && d.Id() == "" {
		// only bind live routes at this stage if we're not doing a blue/green deployment
		if mappedRoutes, err := addRouteMappings(app.ID, v.(*schema.Set).List(), "", rm); err != nil {
			return err
		} else {
			appConfig.routesConfig = mappedRoutes
		}
	}

	timeout := time.Second * time.Duration(d.Get("timeout").(int))
	stopped := d.Get("stopped").(bool)

	// Start application if not stopped
	// state once upload has completed
	if err = <-upload; err != nil {
		return err
	}
	if !stopped {
		if err = am.StartApp(app.ID, timeout); err != nil {
			return err
		}
	}

	if app, err = am.ReadApp(app.ID); err != nil {
		return err
	}
	appConfig.app = app
	session.Log.DebugMessage("Created app state: %# v", app)

	if hasServiceBindings {
		appConfig.serviceBindings = serviceBindings
		session.Log.DebugMessage("Created service bindings: %# v", d.Get("service_binding"))
	}

	return err
}

func resourceAppRead(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	appID := d.Id()
	am := session.AppManager()
	rm := session.RouteManager()

	var app cfapi.CCApp
	if app, err = am.ReadApp(appID); err != nil {
		if strings.Contains(err.Error(), "status code: 404") {
			d.SetId("")
			err = nil
		}
	} else {
		setAppArguments(app, d)

		var serviceBindings []map[string]interface{}
		if serviceBindings, err = am.ReadServiceBindingsByApp(app.ID); err != nil {
			return
		}
		var newStateServiceBindings []map[string]interface{}
		for _, binding := range serviceBindings {
			stateBindingData := make(map[string]interface{})
			stateBindingData["service_instance"] = binding["service_instance"].(string)
			stateBindingData["binding_id"] = binding["binding_id"].(string)
			credentials := binding["credentials"].(map[string]interface{})
			for k, v := range normalizeMap(credentials, make(map[string]interface{}), "", "_") {
				credentials[k] = fmt.Sprintf("%v", v)
			}
			stateBindingData["credentials"] = credentials
			newStateServiceBindings = append(newStateServiceBindings, stateBindingData)
		}
		if len(newStateServiceBindings) > 0 {
			if err := d.Set("service_binding", newStateServiceBindings); err != nil {
				log.Printf("[WARN] Error setting service_binding to cf_app (%s): %s", d.Id(), err)
			}
		}

		if _, hasOldRoute := d.GetOk("route"); hasOldRoute {
			var routeMappings []map[string]interface{}
			if routeMappings, err = rm.ReadRouteMappingsByApp(app.ID); err != nil {
				return
			}
			var stateRouteList = d.Get("route").([]interface{})
			var stateRouteMappings map[string]interface{}
			if len(stateRouteList) == 1 && stateRouteList[0] != nil {
				stateRouteMappings = stateRouteList[0].(map[string]interface{})
			} else {
				stateRouteMappings = make(map[string]interface{})
			}
			currentRouteMappings := make(map[string]interface{})
			for _, r := range []string{
				"default_route",
				"stage_route",
				"live_route",
			} {
				currentRouteMappings[r] = ""
				currentRouteMappings[r+"_mapping_id"] = ""
				for _, mapping := range routeMappings {
					var mappingID, route = mapping["mapping_id"], mapping["route"]
					if route == stateRouteMappings[r] {
						currentRouteMappings[r+"_mapping_id"] = mappingID
						currentRouteMappings[r] = route
						break
					}
				}
			}
			d.Set("route", [...]interface{}{currentRouteMappings})
		} else if srd, hasNewRoutes := d.GetOk("routes"); hasNewRoutes {
			stateRoutes := srd.(*schema.Set)
			if currentRouteMappings, err := rm.ReadRouteMappingsByApp(app.ID); err != nil {
				return err
			} else {
				var updatedRoutes []interface{}
				for _, mapping := range currentRouteMappings {

					refreshedData := map[string]interface{}{
						"mapping_id": mapping["mapping_id"].(string),
						"port":       mapping["port"].(int),
						"route":      mapping["route"].(string),
					}
					if stateRoutes.Contains(refreshedData) {
						updatedRoutes = append(updatedRoutes, refreshedData)
					}
				}
				if err := d.Set("routes", schema.NewSet(hashRouteMappingSet, updatedRoutes)); err != nil {
					return err
				}
			}
		}
	}

	// check if any old deposed resources still exist
	if v, ok := d.GetOk("deposed"); ok {
		deposedResources := v.(map[string]interface{})
		for r, _ := range deposedResources {
			if _, err := am.ReadApp(r); err != nil {
				if strings.Contains(err.Error(), "status code: 404") {
					delete(deposedResources, r)
				}
			} else {
				delete(deposedResources, r)
			}
		}
		if err := d.Set("deposed", deposedResources); err != nil {
			return err
		}
	}

	return err
}

func resourceAppUpdate(d *schema.ResourceData, meta interface{}) (err error) {
	// preseve deposed resources until we clean them up
	existingDeposed, _ := d.GetChange("deposed")
	d.Set("deposed", existingDeposed)

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	// TODO: clean-up old deposed resources

	app := cfapi.CCApp{}

	// Enable partial state mode
	// We need to explicitly set state updates ourselves or
	// tell terraform when a state change is applied and thus okay to persist
	d.Partial(true)

	update := false // for changes where no restart is required
	app.Name = *getChangedValueString("name", &update, d)
	app.SpaceGUID = *getChangedValueString("space", &update, d)
	app.Instances = getChangedValueInt("instances", &update, d)
	app.EnableSSH = getChangedValueBool("enable_ssh", &update, d)
	app.HealthCheckHTTPEndpoint = getChangedValueString("health_check_http_endpoint", &update, d)
	app.HealthCheckType = getChangedValueString("health_check_type", &update, d)
	app.HealthCheckTimeout = getChangedValueInt("health_check_timeout", &update, d)

	restart := false // for changes where just a restart is required
	app.Ports = getChangedValueIntList("ports", &restart, d)
	app.Memory = getChangedValueInt("memory", &restart, d)
	app.DiskQuota = getChangedValueInt("disk_quota", &restart, d)
	app.Command = getChangedValueString("command", &restart, d)

	restage := false // for changes where a full restage is required
	app.Buildpack = getChangedValueString("buildpack", &restage, d)
	app.StackGUID = getChangedValueString("stack", &restage, d)
	app.Environment = getChangedValueMap("environment", &restage, d)

	blueGreen := false
	if v, ok := d.GetOk("blue_green"); ok {
		blueGreenConfig := v.([]interface{})[0].(map[string]interface{})
		if blueGreenEnabled, ok := blueGreenConfig["enable"]; ok && blueGreenEnabled.(bool) {
			if restart || restage || d.HasChange("service_binding") ||
				d.HasChange("url") || d.HasChange("git") || d.HasChange("github_release") || d.HasChange("add_content") {
				blueGreen = true
			}
		}
	}

	if blueGreen {
		if routes, ok := d.GetOk("routes"); !ok || routes.(*schema.Set).Len() < 1 {
			// TODO: add test to ensure this check is done
			return fmt.Errorf("Blue/green mode requires a 'routes' block.")
		}
		err = resourceAppBlueGreenUpdate(d, meta, app)
	} else {
		// fall back to a standard update to the existing app
		err = resourceAppStandardUpdate(d, meta, app, update, restart, restage)
	}

	if err == nil {
		// We succeeded, disable partial mode
		d.Partial(false)
	}

	return err
}

func resourceAppBlueGreenUpdate(d *schema.ResourceData, meta interface{}, newApp cfapi.CCApp) error {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	am := session.AppManager()
	rm := session.RouteManager()

	blueGreenConfig := d.Get("blue_green").([]interface{})[0].(map[string]interface{})

	var venerableApp cfapi.CCApp
	if v, err := am.ReadApp(d.Id()); err != nil {
		return err
	} else {
		venerableApp = v
	}

	// Update origin app name
	if venerableAppRefeshed, err := am.UpdateApp(cfapi.CCApp{ID: d.Id(), Name: venerableApp.Name + "-venerable"}); err != nil {
		return err
	} else {
		venerableApp = venerableAppRefeshed
	}

	appConfig := cfAppConfig{
		app: newApp,
	}
	appConfig.app.Instances = func(i int) *int { return &i }(1) // start the staged app with only one instance (we'll scale it up later)
	if err := resourceAppCreateCfApp(d, meta, &appConfig); err != nil {
		return err
	}
	appConfig.app.Instances = newApp.Instances // restore final expected instances count
	newApp = appConfig.app                     // bring "newApp" var up-to-date, to help prevent bugs

	// TODO: Execute blue-green validation, including mapping staging route(s)!

	// now that we've passed validation, we've passed the point of no return
	d.SetId(appConfig.app.ID)
	d.SetPartial("url")
	d.SetPartial("git")
	d.SetPartial("github_release")
	d.SetPartial("add_content")
	d.SetPartial("service_binding")
	setAppArguments(appConfig.app, d)

	// ensure we keep track of the old application to clean it up later if we fail
	deposedResources := d.Get("deposed").(map[string]interface{})
	deposedResources[venerableApp.ID] = "application"
	d.Set("deposed", deposedResources)

	// Now bind the live routes to the new application instance and scale it up
	if mappedRoutes, err := addRouteMappings(appConfig.app.ID, d.Get("routes").(*schema.Set).List(), venerableApp.ID, rm); err != nil {
		return err
	} else {
		appConfig.routesConfig = mappedRoutes
	}
	d.SetPartial("route")

	var timeoutDuration time.Duration
	if v, ok := d.GetOk("timeout"); ok {
		vv := v.(int)
		timeoutDuration = time.Second * time.Duration(vv)
	}

	shutdownWaitTime := time.Duration(0)
	if v, ok := blueGreenConfig["shutdown_wait"]; ok {
		shutdownWaitTime = time.Duration(v.(int)) * time.Minute
	}
	forgetVenerable := blueGreenConfig["forget_venerable"].(bool)
	noScaleDown := shutdownWaitTime > 0 || forgetVenerable

	// now scale up the new app and scale down the old app
	venerableAppScale := cfapi.CCApp{
		ID:        venerableApp.ID,
		Name:      venerableApp.Name,
		Instances: venerableApp.Instances,
	}
	newAppScale := cfapi.CCApp{
		ID:        appConfig.app.ID,
		Name:      appConfig.app.Name,
		Instances: func(i int) *int { return &i }(1),
	}
	session.Log.DebugMessage("newApp.Instances: %d", *newApp.Instances)
	session.Log.DebugMessage("venerableApp.Instances: %d", *venerableAppScale.Instances)
	for *newAppScale.Instances < *newApp.Instances || (*venerableAppScale.Instances > 1 && !noScaleDown) {
		if *newAppScale.Instances < *newApp.Instances {
			// scale up new
			*newAppScale.Instances++
			session.Log.DebugMessage("Scaling up new app %s to instance count %d", newAppScale.ID, *newAppScale.Instances)
			if _, err := am.UpdateApp(newAppScale); err != nil {
				return err
			}
			if *(appConfig.app.State) != "STOPPED" {
				// wait for the new instance to start
				stateConf := &resource.StateChangeConf{
					Pending: []string{"false"},
					Target:  []string{"true"},
					Refresh: func() (interface{}, string, error) {
						c, err := am.CountRunningAppInstances(newAppScale)
						return new(interface{}), strconv.FormatBool(c >= *newAppScale.Instances), err
					},
					Timeout:      timeoutDuration,
					PollInterval: 5 * time.Second,
				}
				if _, err := stateConf.WaitForState(); err != nil {
					return err
				}
			}
		}

		if !noScaleDown {
			if *venerableAppScale.Instances > 1 {
				// scale down old
				*venerableAppScale.Instances--
				session.Log.DebugMessage("Scaling down venerable app %s to instance count %d", venerableAppScale.ID, *venerableAppScale.Instances)
				if _, err := am.UpdateApp(venerableAppScale); err != nil {
					return err
				}
				if *venerableApp.State != "STOPPED" {
					// wait for the instance to stop
					stateConf := &resource.StateChangeConf{
						Pending: []string{"false"},
						Target:  []string{"true"},
						Refresh: func() (interface{}, string, error) {
							c, err := am.CountRunningAppInstances(venerableApp)
							return new(interface{}), strconv.FormatBool(c <= *venerableApp.Instances), err
						},
						Timeout:      timeoutDuration,
						PollInterval: 5 * time.Second,
					}
					if _, err := stateConf.WaitForState(); err != nil {
						return err
					}
					// CF gives shutting down processes at most 10 seconds to exit
					time.Sleep(time.Second * time.Duration(10))
				}
			}
		} else {
			session.Log.DebugMessage("Not scaling down venerable app (%s) due to a configured shutdown_wait=%dm or forget_venerable=%t",
				venerableApp.ID, blueGreenConfig["shutdown_wait"].(int), forgetVenerable)
		}
	}

	// delete mappings from the venerable application
	oldRoutes, _ := d.GetChange("routes")
	if oldRoutesSet := oldRoutes.(*schema.Set); oldRoutesSet.Len() > 0 {
		session.Log.DebugMessage("Deleting venerable app route mappings: %v", oldRoutesSet)
		if err := deleteRouteMappings(oldRoutesSet.List(), rm); err != nil {
			return err
		}
	}

	waitCyclePeriod := time.Second * time.Duration(10)
	if shutdownWaitTime > 0 {
		for waited := time.Duration(0); waited < shutdownWaitTime; waited = waited + waitCyclePeriod {
			session.Log.DebugMessage("Waiting for venerable app (%s) shutdown_wait period to expire... (waited=%ds) (shutdown_wait=%dm)",
				venerableApp.ID, waited/time.Second, shutdownWaitTime/time.Minute)
			time.Sleep(waitCyclePeriod)
		}
	}

	if !forgetVenerable {
		// now delete the venerable application
		if err := am.DeleteApp(venerableAppScale.ID, true); err != nil {
			return err
		}
	}

	// if we get this far, the venerable app is no longer considered
	// a dangling deposed resource, even if it still exists
	delete(deposedResources, venerableApp.ID)
	d.Set("deposed", deposedResources)

	// TODO: unmap stage route

	return nil
}

func resourceAppStandardUpdate(d *schema.ResourceData, meta interface{}, app cfapi.CCApp, update bool, restart bool, restage bool) error {
	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	am := session.AppManager()
	rm := session.RouteManager()

	app.ID = d.Id()

	if update || restart || restage {
		// push any updates to CF, we'll do any restage/restart later
		var err error
		if app, err = am.UpdateApp(app); err != nil {
			return err
		}
		setAppArguments(app, d)
		d.SetPartial("name")
		d.SetPartial("space")
		d.SetPartial("ports")
		d.SetPartial("instances")
		d.SetPartial("memory")
		d.SetPartial("disk_quota")
		d.SetPartial("command")
		d.SetPartial("enable_ssh")
		d.SetPartial("health_check_http_endpoint")
		d.SetPartial("health_check_type")
		d.SetPartial("health_check_timeout")
		d.SetPartial("buildpack")
		d.SetPartial("environment")
	}

	// update the application's service bindings (the necessary restage is dealt with later)
	if d.HasChange("service_binding") {

		old, new := d.GetChange("service_binding")
		session.Log.DebugMessage("Old service bindings state:: %# v", old)
		session.Log.DebugMessage("New service bindings state:: %# v", new)

		bindingsToDelete, bindingsToAdd := getListChangedSchemaLists(old.([]interface{}), new.([]interface{}))
		session.Log.DebugMessage("Service bindings to be deleted: %# v", bindingsToDelete)
		session.Log.DebugMessage("Service bindings to be added: %# v", bindingsToAdd)

		if err := removeServiceBindings(bindingsToDelete, am, session.Log); err != nil {
			return err
		}

		if added, err := addServiceBindings(app.ID, bindingsToAdd, am, session.Log); err != nil {
			return err
		} else if len(added) > 0 {
			if new != nil {
				for _, b := range new.([]interface{}) {
					bb := b.(map[string]interface{})

					for _, a := range added {
						if bb["service_instance"] == a["service_instance"] {
							bb["binding_id"] = a["binding_id"]
							bb["credentials"] = a["credentials"]
							break
						}
					}
				}
				d.Set("service_binding", new)
			}
		}
		// the changes were applied, in CF even though they might not have taken effect
		// in the application, we'll allow the state updates for this property to occur
		d.SetPartial("service_binding")
		restage = true
	}

	if d.HasChange("route") {
		if !d.HasChange("routes") {
			// still using the old "route" block
			session.Log.DebugMessage("Updating based on old style 'route' block (app=%s)", app.ID)
			old, new := d.GetChange("route")

			var (
				oldRouteConfig, newRouteConfig map[string]interface{}
			)

			oldA := old.([]interface{})
			if len(oldA) == 1 {
				oldRouteConfig = oldA[0].(map[string]interface{})
			} else {
				oldRouteConfig = make(map[string]interface{})
			}
			newA := new.([]interface{})
			if len(newA) == 1 {
				newRouteConfig = newA[0].(map[string]interface{})
			} else {
				newRouteConfig = make(map[string]interface{})
			}

			for _, r := range []string{
				"default_route",
				"stage_route",
				"live_route",
			} {
				if oldRouteConfig[r] == newRouteConfig[r] {
					continue
				}
				if _, err := validateRouteLegacy(newRouteConfig, r, app.ID, rm); err != nil {
					return err
				}
				if mappingID, err := updateAppRouteMappings(oldRouteConfig, newRouteConfig, r, app.ID, rm); err != nil {
					return err
				} else if len(mappingID) > 0 {
					newRouteConfig[r+"_mapping_id"] = mappingID
				}
			}
			d.Set("route", [...]interface{}{newRouteConfig})
		} else {
			// this means a new style "routes" block replaced the old "route" block
			session.Log.DebugMessage("Migrating from 'route' block to 'routes' block (app=%s)", app.ID)
			oldRoute, _ := d.GetChange("route")
			_, newRoutes := d.GetChange("routes")

			var oldRouteConfig map[string]interface{}
			oldA := oldRoute.([]interface{})
			if len(oldA) == 1 {
				oldRouteConfig = oldA[0].(map[string]interface{})
			} else {
				oldRouteConfig = make(map[string]interface{})
			}

			newRouteConfig := newRoutes.(*schema.Set)

			routesList := newRouteConfig.List()
			for i, r := range routesList {
				data := r.(map[string]interface{})
				matchingOldRouteFound := false
				for _, r := range []string{
					"default_route",
					"stage_route",
					"live_route",
				} {
					if oldRouteConfig[r].(string) == data["route"].(string) {
						data["mapping_id"] = oldRouteConfig[r+"_mapping_id"].(string)
						matchingOldRouteFound = true
						break
					}
				}
				if !matchingOldRouteFound {
					routeID := data["route"].(string)
					if err := validateRoute(app.ID, routeID, rm); err != nil {
						return err
					}
					if mappingID, err := rm.CreateRouteMapping(routeID, app.ID, nil); err != nil {
						return err
					} else {
						data["mapping_id"] = mappingID
					}
				}
				// read mapping port
				if mapping, err := rm.ReadRouteMapping(data["mapping_id"].(string)); err != nil {
					return err
				} else {
					data["port"] = mapping.AppPort
				}
				routesList[i] = data
			}
			if err := d.Set("routes", schema.NewSet(hashRouteMappingSet, routesList)); err != nil {
				return err
			}
			d.SetPartial("routes")
		}
		d.SetPartial("route")
	} else if d.HasChange("routes") {
		// handle updates for a new style "routes" block only
		session.Log.DebugMessage("Updating routes based on new style 'routes' block (app=%s)", app.ID)

		o, n := d.GetChange("routes")
		if o == nil {
			o = new(schema.Set)
		}
		if n == nil {
			n = new(schema.Set)
		}
		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		// in case of partial updates we need to keep track of all the mappings we
		// added and all those we failed to remove
		updatedRoutes := os

		// mappings to add
		for _, r := range ns.Difference(os).List() {
			data := r.(map[string]interface{})
			routeID := data["route"].(string)
			if err := validateRoute(app.ID, routeID, rm); err != nil {
				return err
			}
			if mappingID, err := rm.CreateRouteMapping(routeID, app.ID, nil); err != nil {
				return err
			} else {
				data["mapping_id"] = mappingID
				updatedRoutes.Add(data)
				if err := d.Set("routes", updatedRoutes); err != nil {
					return err
				}
			}
			// read mapping port
			if mapping, err := rm.ReadRouteMapping(data["mapping_id"].(string)); err != nil {
				return err
			} else {
				data["port"] = mapping.AppPort
				// re-add it with the new data
				updatedRoutes.Remove(data)
				updatedRoutes.Add(data)
				if err := d.Set("routes", updatedRoutes); err != nil {
					return err
				}
			}
		}

		// mappings to remove
		for _, r := range os.Difference(ns).List() {
			data := r.(map[string]interface{})
			if mappingID, ok := data["mapping_id"].(string); ok && len(mappingID) > 0 {
				if err := rm.DeleteRouteMapping(mappingID); err != nil {
					if !strings.Contains(err.Error(), "status code: 404") {
						return err
					}
				}
				updatedRoutes.Remove(r)
				if err := d.Set("routes", updatedRoutes); err != nil {
					return err
				}
			}
		}

		// mappings which may need updating
		// TODO: need to implement this in order to handle the port and exclusive fields
		/* oldDataList := os.Intersection(ns).List()
		for i, r := range ns.Intersection(os).List() {
			oldData := oldDataList[i].(map[string]interface{})
			newData := r.(map[string]interface{})

			if !reflect.DeepEqual(oldData, newData) {

			}
		} */
		d.SetPartial("routes")
	}

	binaryUpdated := false // check if we need to update the application's binary
	if d.HasChange("url") || d.HasChange("git") || d.HasChange("github_release") || d.HasChange("add_content") {

		var (
			v  interface{}
			ok bool

			appPath string

			addContent []map[string]interface{}
		)

		appPathChan, errChan := prepareApp(app, d, session.Log)
		appPath = <-appPathChan
		if err := <-errChan; err != nil {
			return err
		}

		if v, ok = d.GetOk("add_content"); ok {
			addContent = getListOfStructs(v)
		}

		if err := am.UploadApp(app, appPath, addContent); err != nil {
			return err
		}
		binaryUpdated = true
	}

	// now that all of the reconfiguration is done, we can deal doing a restage or restart, as required
	timeout := time.Second * time.Duration(d.Get("timeout").(int))

	// check the package state of the application after binary upload
	var curApp cfapi.CCApp
	var readErr error
	if curApp, readErr = am.ReadApp(app.ID); readErr != nil {
		return readErr
	}
	if binaryUpdated || restage {
		// There seem to be more types of updates that can automagically put an app's package_stage into "PENDING"
		// for right now, I have observed this after a service binding update as well, but I have no idea what other
		// optierations might cause this.  For now, we'll just do a blanket check since calling restage when the app
		// is in this state causes the API to throw an error.
		time.Sleep(time.Second * time.Duration(5)) // pause for a few seconds here to ensure the CF API has caught up
		if *curApp.PackageState != "PENDING" {
			// if it's not already pending, we need to restage
			restage = true
		} else {
			// uploading the binary flagged the app for restaging,
			// but we need to restart in order to force that to happen now
			// (this is how the CF CLI does this)
			restage = false
			restart = true
		}
	}

	if restage {
		if err := am.RestageApp(app.ID, timeout); err != nil {
			return err
		}
		if *curApp.State == "STARTED" {
			// if the app was running before the restage when wait for it to start again
			if err := am.WaitForAppToStart(app, timeout); err != nil {
				return err
			}
		}
	} else if restart && !d.Get("stopped").(bool) { // only run restart if the final state is running
		if err := am.StopApp(app.ID, timeout); err != nil {
			return err
		}
		if err := am.StartApp(app.ID, timeout); err != nil {
			return err
		}
	}

	// now set the final started/stopped state, whatever it is
	if d.HasChange("stopped") {
		if d.Get("stopped").(bool) {
			if err := am.StopApp(app.ID, timeout); err != nil {
				return err
			}
		} else {
			if err := am.StartApp(app.ID, timeout); err != nil {
				return err
			}
		}
	}

	return nil
}

func resourceAppDelete(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	am := session.AppManager()
	rm := session.RouteManager()

	if v, ok := d.GetOk("service_binding"); ok {
		if err = removeServiceBindings(getListOfStructs(v), am, session.Log); err != nil {
			return
		}
	}
	if v, ok := d.GetOk("route"); ok {

		routeConfig := v.([]interface{})[0].(map[string]interface{})

		for _, r := range []string{
			"default_route_mapping_id",
			"stage_route_mapping_id",
			"live_route_mapping_id",
		} {
			if v, ok := routeConfig[r]; ok {
				mappingID := v.(string)
				if len(mappingID) > 0 {
					if err = rm.DeleteRouteMapping(v.(string)); err != nil {
						if !strings.Contains(err.Error(), "status code: 404") {
							return
						}
						err = nil
					}
				}
			}
		}
	}
	if v, ok := d.GetOk("routes"); ok {
		if err = deleteRouteMappings(v.(*schema.Set).List(), rm); err != nil {
			return err
		}
	}
	err = am.DeleteApp(d.Id(), false)
	if err = am.DeleteApp(d.Id(), false); err != nil {
		if strings.Contains(err.Error(), "status code: 404") {
			session.Log.DebugMessage(
				"Application with ID '%s' does not exist. App resource will be deleted from state",
				d.Id())
		} else {
			session.Log.DebugMessage(
				"App resource will be deleted from state although deleting app with ID '%s' returned an error: %s",
				d.Id(), err.Error())
		}
	}
	return nil
}

func setAppArguments(app cfapi.CCApp, d *schema.ResourceData) {

	d.Set("name", app.Name)
	d.Set("space", app.SpaceGUID)
	if app.Instances != nil || IsImportState(d) {
		d.Set("instances", app.Instances)
	}
	if app.Memory != nil || IsImportState(d) {
		d.Set("memory", app.Memory)
	}
	if app.DiskQuota != nil || IsImportState(d) {
		d.Set("disk_quota", app.DiskQuota)
	}
	if app.StackGUID != nil || IsImportState(d) {
		d.Set("stack", app.StackGUID)
	}
	if app.Buildpack != nil || IsImportState(d) {
		d.Set("buildpack", app.Buildpack)
	}
	if app.Command != nil || IsImportState(d) {
		d.Set("command", app.Command)
	}
	if app.EnableSSH != nil || IsImportState(d) {
		d.Set("enable_ssh", app.EnableSSH)
	}
	if app.HealthCheckHTTPEndpoint != nil || IsImportState(d) {
		d.Set("health_check_http_endpoint", app.HealthCheckHTTPEndpoint)
	}
	if app.HealthCheckType != nil || IsImportState(d) {
		d.Set("health_check_type", app.HealthCheckType)
	}
	if app.HealthCheckTimeout != nil || IsImportState(d) {
		d.Set("health_check_timeout", app.HealthCheckTimeout)
	}
	if app.Environment != nil || IsImportState(d) {
		d.Set("environment", app.Environment)
	}

	d.SetPartial("timeout")
	d.Set("stopped", *app.State != "STARTED")

	ports := []interface{}{}
	for _, p := range *app.Ports {
		ports = append(ports, p)
	}
	d.Set("ports", schema.NewSet(resourceIntegerSet, ports))
}

func prepareApp(app cfapi.CCApp, d *schema.ResourceData, log *cfapi.Logger) (<-chan string, <-chan error) {
	pathChan := make(chan string, 1)
	errChan := make(chan error, 1)

	if v, ok := d.GetOk("url"); ok {
		go func() {
			var path string
			var err error
			url := v.(string)

			if strings.HasPrefix(url, "file://") {
				path = url[7:]
			} else {

				var (
					resp *http.Response

					in  io.ReadCloser
					out *os.File
				)

				if out, err = ioutil.TempFile("", "cfapp"); err == nil {
					log.UI.Say("Downloading application %s from url %s.", terminal.EntityNameColor(app.Name), url)
					if resp, err = http.Get(url); err == nil {
						in = resp.Body
						if _, err = io.Copy(out, in); err == nil {
							if err = out.Close(); err == nil {
								path = out.Name()
							}
						}
					}
				}
			}
			log.UI.Say("Application downloaded to: %s", path)
			pathChan <- path
			errChan <- err
			close(pathChan)
			close(errChan)
			return
		}()

	} else {
		log.UI.Say("Retrieving application %s source / binary.", terminal.EntityNameColor(app.Name))

		_, isGithubRelease := d.GetOk("github_release")
		repositoryChan, repoErrChan := getRepositoryFromConfigAsync(d)
		go func() {
			var path string
			repository := <-repositoryChan
			err := <-repoErrChan
			if err != nil {
				return
			}

			if isGithubRelease {
				path = filepath.Dir(repository.GetPath())
			} else {
				path = repository.GetPath()
			}
			log.UI.Say("Application downloaded to: %s", path)
			pathChan <- path
			errChan <- err
			close(pathChan)
			close(errChan)
		}()
	}

	return pathChan, errChan
}

func validateRoute(appID string, routeID string, rm *cfapi.RouteManager) error {
	if mappings, err := rm.ReadRouteMappingsByRoute(routeID); err == nil && len(mappings) > 0 {
		if len(mappings) == 1 {
			if boundApp, ok := mappings[0]["app"]; ok && boundApp == appID {
				return nil
			}
		}
		return fmt.Errorf(
			"route with id %s is already mapped. routes specificed in the 'routes' argument can only be mapped to one 'cf_app' resource",
			routeID)
	} else {
		return err
	}
}

func addRouteMappings(appID string, routes []interface{}, validCurrentAppMapping string, rm *cfapi.RouteManager) ([]interface{}, error) {
	var mappedRoutes []interface{}
	for _, r := range routes {
		data := r.(map[string]interface{})
		routeID := data["route"].(string)
		if err := validateRoute(validCurrentAppMapping, routeID, rm); err != nil {
			return nil, err
		}
		if mappingID, err := rm.CreateRouteMapping(routeID, appID, nil); err != nil {
			return nil, err
		} else {
			data["mapping_id"] = mappingID
		}
		// read mapping port
		if mapping, err := rm.ReadRouteMapping(data["mapping_id"].(string)); err != nil {
			return nil, err
		} else {
			data["port"] = mapping.AppPort
		}
		mappedRoutes = append(mappedRoutes, data)
	}
	return mappedRoutes, nil
}

func deleteRouteMappings(routes []interface{}, rm *cfapi.RouteManager) error {
	for _, r := range routes {
		data := r.(map[string]interface{})
		if mappingID, ok := data["mapping_id"].(string); ok && len(mappingID) > 0 {
			if err := rm.DeleteRouteMapping(mappingID); err != nil {
				if !strings.Contains(err.Error(), "status code: 404") {
					return err
				}
			}
		}
	}
	return nil
}

func validateRouteLegacy(routeConfig map[string]interface{}, route string, appID string, rm *cfapi.RouteManager) (routeID string, err error) {

	if v, ok := routeConfig[route]; ok {

		routeID = v.(string)

		var mappings []map[string]interface{}
		if mappings, err = rm.ReadRouteMappingsByRoute(routeID); err == nil && len(mappings) > 0 {
			if len(mappings) == 1 {
				if app, ok := mappings[0]["app"]; ok && app == appID {
					return routeID, err
				}
			}
			err = fmt.Errorf(
				"route with id %s is already mapped. routes specificed in the 'route' argument can only be mapped to one 'cf_app' resource",
				routeID)
		}
	}
	return routeID, err
}

func updateAppRouteMappings(
	old map[string]interface{},
	new map[string]interface{},
	route, appID string, rm *cfapi.RouteManager) (mappingID string, err error) {

	var (
		oldRouteID, newRouteID string
	)

	if v, ok := old[route]; ok {
		oldRouteID = v.(string)
	}
	if v, ok := new[route]; ok {
		newRouteID = v.(string)
	}

	if oldRouteID != newRouteID {
		if len(newRouteID) > 0 {
			if mappingID, err = rm.CreateRouteMapping(newRouteID, appID, nil); err != nil {
				return "", err
			}
		}
		if len(oldRouteID) > 0 {
			if v, ok := old[route+"_mapping_id"]; ok {
				if err = rm.DeleteRouteMapping(v.(string)); err != nil {
					if strings.Contains(err.Error(), "status code: 404") {
						err = nil
					} else {
						return "", err
					}
				}
			}
		}
		if err != nil {
			// this means we failed to delete the old route mapping!
			// TODO: is there anything we can do about this here?
		}
	}
	return mappingID, err
}

func addServiceBindings(
	id string,
	add []map[string]interface{},
	am *cfapi.AppManager,
	log *cfapi.Logger) (bindings []map[string]interface{}, err error) {

	var (
		serviceInstanceID, bindingID string
		params                       *map[string]interface{}

		credentials        map[string]interface{}
		bindingCredentials map[string]interface{}
	)

	for _, b := range add {
		serviceInstanceID = b["service_instance"].(string)
		params = nil
		if v, ok := b["params"]; ok {
			vv := v.(map[string]interface{})
			params = &vv
		}
		if bindingID, bindingCredentials, err = am.CreateServiceBinding(id, serviceInstanceID, params); err != nil {
			return bindings, err
		}
		b["binding_id"] = bindingID

		credentials = b["credentials"].(map[string]interface{})
		for k, v := range normalizeMap(bindingCredentials, make(map[string]interface{}), "", "_") {
			credentials[k] = v
		}

		bindings = append(bindings, b)
		log.DebugMessage("Created binding with id '%s' for service instance '%s'.", bindingID, serviceInstanceID)
	}
	return bindings, nil
}

func removeServiceBindings(delete []map[string]interface{},
	am *cfapi.AppManager, log *cfapi.Logger) error {

	for _, b := range delete {

		serviceInstanceID := b["service_instance"].(string)
		bindingID := b["binding_id"].(string)

		if len(bindingID) > 0 {
			log.DebugMessage("Deleting binding with id '%s' for service instance '%s'.", bindingID, serviceInstanceID)
			if err := am.DeleteServiceBinding(bindingID); err != nil {
				return err
			}
		} else {
			log.DebugMessage("Ignoring binding for service instance '%s' as no corresponding binding id was found.", serviceInstanceID)
		}
	}
	return nil
}
