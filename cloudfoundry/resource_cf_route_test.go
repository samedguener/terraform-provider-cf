package cloudfoundry

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cli/cf/errors"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"github.com/terraform-providers/terraform-provider-cf/cloudfoundry/cfapi"
)

const routeResource = `

data "cf_domain" "local" {
    name = "%s"
}
data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}

resource "cf_app" "test-app-8080" {
	name = "test-app"
	space = "${data.cf_space.space.id}"
	command = "test-app --ports=8080"
	timeout = 1800

	git {
		url = "https://github.com/mevansam/test-app.git"
	}
}
resource "cf_route" "test-app-route" {
	domain = "${data.cf_domain.local.id}"
	space = "${data.cf_space.space.id}"
	hostname = "test-app-single"

	target {
		app = "${cf_app.test-app-8080.id}"
	}
}
`

const routeResourceUpdate = `

data "cf_domain" "local" {
    name = "%s"
}
data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}

resource "cf_app" "test-app-8080" {
	name = "test-app-8080"
	space = "${data.cf_space.space.id}"
	command = "test-app --ports=8080"
	timeout = 1800

	git {
		url = "https://github.com/mevansam/test-app.git"
	}
}
resource "cf_app" "test-app-8888" {
	name = "test-app-8888"
	space = "${data.cf_space.space.id}"
	ports = [ 8888 ]
	command = "test-app --ports=8888"
	timeout = 1800

	git {
		url = "https://github.com/mevansam/test-app.git"
	}
}
resource "cf_app" "test-app-9999" {
	name = "test-app-9999"
	space = "${data.cf_space.space.id}"
	ports = [ 9999 ]
	command = "test-app --ports=9999"
	timeout = 1800

	git {
		url = "https://github.com/mevansam/test-app.git"
	}
}
resource "cf_route" "test-app-route" {
	domain = "${data.cf_domain.local.id}"
	space = "${data.cf_space.space.id}"
	hostname = "test-app-multi"

	target {
		app = "${cf_app.test-app-9999.id}"
		port = 9999
	}
	target {
		app = "${cf_app.test-app-8888.id}"
		port = 8888
	}
	target {
		app = "${cf_app.test-app-8080.id}"
	}
}
`

const multipleRoute = `

data "cf_domain" "local" {
    name = "%s"
}
data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}
data "cf_service" "mysql" {
    name = "p-mysql"
}
data "cf_service" "rmq" {
    name = "p-rabbitmq"
}

resource "cf_route" "spring-music-base" {
	domain = "${data.cf_domain.local.id}"
	space = "${data.cf_space.space.id}"
	hostname = "spring-music"
    target = {app = "${cf_app.spring-music.id}"}
}
resource "cf_route" "spring-music" {
	domain = "${data.cf_domain.local.id}"
	space = "${data.cf_space.space.id}"
	hostname = "spring-music"
    path = "/api/v2/fizzbuzz/"  
    target = {app = "${cf_app.spring-music.id}"}
}
resource "cf_service_instance" "db" {
	name = "db"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.mysql.service_plans.512mb}"
}
resource "cf_service_instance" "fs1" {
	name = "fs1"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.rmq.service_plans.standard}"
}
resource "cf_app" "spring-music" {
	name = "spring-music"
	space = "${data.cf_space.space.id}"
	memory = "768"
	disk_quota = "512"
	timeout = 1800

	url = "https://github.com/mevansam/spring-music/releases/download/v1.0/spring-music.war"

	service_binding {
		service_instance = "${cf_service_instance.db.id}"
	}
	service_binding {
		service_instance = "${cf_service_instance.fs1.id}"
	}

	environment {
		TEST_VAR_1 = "testval1"
		TEST_VAR_2 = "testval2"
	}
}
`

const multipleRouteUpdate = `

data "cf_domain" "local" {
    name = "%s"
}
data "cf_org" "org" {
    name = "pcfdev-org"
}
data "cf_space" "space" {
    name = "pcfdev-space"
	org = "${data.cf_org.org.id}"
}
data "cf_service" "mysql" {
    name = "p-mysql"
}
data "cf_service" "rmq" {
    name = "p-rabbitmq"
}

resource "cf_route" "spring-music-base" {
	domain = "${data.cf_domain.local.id}"
	space = "${data.cf_space.space.id}"
	hostname = "spring-music"
    target = {app = "${cf_app.spring-music.id}"}
}
resource "cf_route" "spring-music" {
	domain = "${data.cf_domain.local.id}"
	space = "${data.cf_space.space.id}"
	hostname = "spring-music"
    path = "/api/v2/fizzbuzz/"  
    target = {app = "${cf_app.spring-music.id}"}
}
resource "cf_service_instance" "db" {
	name = "db"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.mysql.service_plans.512mb}"
}
resource "cf_service_instance" "fs1" {
	name = "fs1"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.rmq.service_plans.standard}"
}
resource "cf_service_instance" "fs2" {
	name = "fs2"
    space = "${data.cf_space.space.id}"
    service_plan = "${data.cf_service.rmq.service_plans.standard}"
}
resource "cf_app" "spring-music" {
	name = "spring-music-updated"
	space = "${data.cf_space.space.id}"
	memory = "1024"
	disk_quota = "1024"
	timeout = 1800

	url = "https://github.com/mevansam/spring-music/releases/download/v1.0/spring-music.war"

	service_binding {
		service_instance = "${cf_service_instance.db.id}"
	}
	service_binding {
		service_instance = "${cf_service_instance.fs2.id}"
	}
	service_binding {
		service_instance = "${cf_service_instance.fs1.id}"
	}

	environment {
		TEST_VAR_1 = "testval1"
		TEST_VAR_2 = "testval2"
	}
}
`

func TestAccRoute_multiple(t *testing.T) {

	refRouteBase := "cf_route.spring-music-base"
	refRoute := "cf_route.spring-music"

	resource.Test(t,
		resource.TestCase{
			PreCheck:     func() { testAccPreCheck(t) },
			Providers:    testAccProviders,
			CheckDestroy: testAccCheckAppDestroyed([]string{"spring-music"}),
			Steps: []resource.TestStep{


				resource.TestStep{
					Config: fmt.Sprintf(multipleRoute, defaultAppDomain()),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckRouteExists(refRoute, func() (err error) {

							if err = assertHTTPResponse("https://spring-music."+defaultAppDomain(), 200, nil); err != nil {
								return err
							}
							return
						}),
						testAccCheckRouteExists(refRouteBase, func() (err error) {

							if err = assertHTTPResponse("https://spring-music."+defaultAppDomain(), 200, nil); err != nil {
								return err
							}
							return
						}),
					),
				},

				resource.TestStep{
					Config: fmt.Sprintf(multipleRouteUpdate, defaultAppDomain()),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckRouteExists(refRoute, func() (err error) {

							if err = assertHTTPResponse("https://spring-music."+defaultAppDomain(), 200, nil); err != nil {
								return err
							}
							return
						}),
						testAccCheckRouteExists(refRouteBase, func() (err error) {

							if err = assertHTTPResponse("https://spring-music."+defaultAppDomain(), 200, nil); err != nil {
								return err
							}
							return
						}),
					),
				},
			},
		})
}

func TestAccRoute_normal(t *testing.T) {

	refRoute := "cf_route.test-app-route"

	resource.Test(t,
		resource.TestCase{
			PreCheck:     func() { testAccPreCheck(t) },
			Providers:    testAccProviders,
			CheckDestroy: testAccCheckRouteDestroyed([]string{"test-app-single", "test-app-multi"}, defaultAppDomain()),
			Steps: []resource.TestStep{

				resource.TestStep{
					Config: fmt.Sprintf(routeResource, defaultAppDomain()),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckRouteExists(refRoute, func() (err error) {

							responses := []string{"8080"}
							if err = assertHTTPResponse("http://test-app-single."+defaultAppDomain()+"/port", 200, &responses); err != nil {
								return err
							}
							return
						}),
						resource.TestCheckResourceAttr(
							refRoute, "hostname", "test-app-single"),
						resource.TestCheckResourceAttr(
							refRoute, "target.#", "1"),
					),
				},

				resource.TestStep{
					Config: fmt.Sprintf(routeResourceUpdate, defaultAppDomain()),
					Check: resource.ComposeTestCheckFunc(
						testAccCheckRouteExists(refRoute, func() (err error) {

							responses := []string{"8080", "8888", "9999"}
							for i := 1; i <= 9; i++ {
								if err = assertHTTPResponse("http://test-app-multi."+defaultAppDomain()+"/port", 200, &responses); err != nil {
									return err
								}
							}
							return
						}),
						resource.TestCheckResourceAttr(
							refRoute, "hostname", "test-app-multi"),
						resource.TestCheckResourceAttr(
							refRoute, "target.#", "3"),
					),
				},
			},
		})
}

func testAccCheckRouteExists(resRoute string, validate func() error) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {

		session := testAccProvider.Meta().(*cfapi.Session)

		rs, ok := s.RootModule().Resources[resRoute]
		if !ok {
			return fmt.Errorf("route '%s' not found in terraform state", resRoute)
		}

		session.Log.DebugMessage(
			"terraform state for resource '%s': %# v",
			resRoute, rs)

		id := rs.Primary.ID
		attributes := rs.Primary.Attributes

		var route cfapi.CCRoute
		rm := session.RouteManager()
		if route, err = rm.ReadRoute(id); err != nil {
			return
		}
		session.Log.DebugMessage(
			"retrieved route for resource '%s' with id '%s': %# v",
			resRoute, id, route)

		if err = assertEquals(attributes, "domain", route.DomainGUID); err != nil {
			return
		}
		if err = assertEquals(attributes, "space", route.SpaceGUID); err != nil {
			return
		}
		if err = assertEquals(attributes, "hostname", route.Hostname); err != nil {
			return
		}
		if err = assertEquals(attributes, "port", route.Port); err != nil {
			return
		}
		if err = assertEquals(attributes, "path", route.Path); err != nil {
			return
		}

		err = validate()
		return
	}
}

func testAccCheckRouteDestroyed(hostnames []string, domain string) resource.TestCheckFunc {

	return func(s *terraform.State) error {

		session := testAccProvider.Meta().(*cfapi.Session)
		for _, h := range hostnames {
			if _, err := session.RouteManager().FindRoute(domain, &h, nil, nil); err != nil {
				switch err.(type) {
				case *errors.ModelNotFoundError:
					continue
				default:
					return err
				}
			}
			return fmt.Errorf("route with hostname '%s' and domain '%s' still exists in cloud foundry", h, domain)
		}
		return nil
	}
}
