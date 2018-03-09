package cloudfoundry

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/terraform-providers/terraform-provider-cf/cloudfoundry/cfapi"
)

func resourceDomain() *schema.Resource {

	return &schema.Resource{

		Create: resourceDomainCreate,
		Read:   resourceDomainRead,
		Delete: resourceDomainDelete,
		Importer: &schema.ResourceImporter{
			State: resourceDomainImport,
		},

		Schema: map[string]*schema.Schema{

			"name": &schema.Schema{
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
				Computed: true,
			},
			"sub_domain": &schema.Schema{
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
				Computed: true,
			},
			"domain": &schema.Schema{
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
				Computed: true,
			},
			"router_group": &schema.Schema{
				Type:          schema.TypeString,
				ForceNew:      true,
				Optional:      true,
				ConflictsWith: []string{"org"},
			},
			"router_type": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"org": &schema.Schema{
				Type:          schema.TypeString,
				ForceNew:      true,
				Optional:      true,
				ConflictsWith: []string{"router_group"},
			},
			// "shared-with": &schema.Schema{
			// 	Type:     schema.TypeSet,
			// 	Optional: true,
			// 	Elem:     &schema.Schema{Type: schema.TypeString},
			// 	Set:      resourceStringHash,
			// },
		},
	}
}

func resourceDomainCreate(d *schema.ResourceData, meta interface{}) error {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	nameAttr, nameOk := d.GetOk("name")
	subDomainAttr, subDomainOk := d.GetOk("sub_domain")
	domainAttr, domainOk := d.GetOk("domain")
	org, orgOk := d.GetOk("org")
	routerGroup, routerGroupOk := d.GetOk("router_group")

	if nameOk {

		domainParts := strings.Split(nameAttr.(string), ".")
		if len(domainParts) <= 1 {
			return fmt.Errorf("the 'name' attribute does not contain a sub-domain")
		}
		sd := domainParts[0]
		dn := strings.Join(domainParts[1:], ".")

		if subDomainOk {
			return fmt.Errorf("the 'sub_domain' will be computed from the 'name' attribute, so it is not needed here")
		}
		if domainOk {
			return fmt.Errorf("the 'domain' will be computed from the 'name' attribute, so it is not needed here")
		}
		d.Set("sub_domain", sd)
		d.Set("domain", dn)
	} else {
		if !subDomainOk || !domainOk {
			return fmt.Errorf("to compute the 'name' both the 'sub_domain' and 'domain' attributes need to be provided")
		}
		d.Set("name", subDomainAttr.(string)+"."+domainAttr.(string))
	}

	var (
		ccDomain cfapi.CCDomain
		err      error
	)
	name := d.Get("name").(string)

	dm := session.DomainManager()
	if orgOk {
		ccDomain, err = dm.CreatePrivateDomain(name, org.(string))
	} else {
		if routerGroupOk {
			rg := routerGroup.(string)
			ccDomain, err = dm.CreateSharedDomain(name, &rg)
			d.Set("router_type", ccDomain.RouterType)
		} else {
			ccDomain, err = dm.CreateSharedDomain(name, nil)
		}
	}
	if err != nil {
		return err
	}
	d.SetId(ccDomain.ID)
	return nil
}

func resourceDomainRead(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	dm := session.DomainManager()
	id := d.Id()

	var ccDomain cfapi.CCDomain

	ccDomain, err = dm.GetSharedDomain(id)
	if err == nil {
		domainParts := strings.Split(ccDomain.Name, ".")
		subDomain := domainParts[0]
		domain := strings.Join(domainParts[1:], ".")

		d.Set("name", ccDomain.Name)
		d.Set("sub_domain", subDomain)
		d.Set("domain", domain)
		d.Set("route_group", ccDomain.RouterGroupGUID)
		d.Set("router_type", ccDomain.RouterType)

		return
	}
	ccDomain, err = dm.GetPrivateDomain(id)
	if err == nil {
		domainParts := strings.Split(ccDomain.Name, ".")
		subDomain := domainParts[0]
		domain := strings.Join(domainParts[1:], ".")

		d.Set("name", ccDomain.Name)
		d.Set("sub_domain", subDomain)
		d.Set("domain", domain)
		d.Set("org", ccDomain.OwningOrganizationGUID)

		return
	}

	return nil
}

func resourceDomainDelete(d *schema.ResourceData, meta interface{}) (err error) {

	session := meta.(*cfapi.Session)
	if session == nil {
		return fmt.Errorf("client is nil")
	}

	dm := session.DomainManager()
	id := d.Id()

	if _, orgOk := d.GetOk("org"); orgOk {
		err = dm.DeletePrivateDomain(id)
	} else {
		err = dm.DeleteSharedDomain(id)
	}
	return
}

func resourceDomainImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	session := meta.(*cfapi.Session)
	var ccDomain cfapi.CCDomain

	if session == nil {
		return nil, fmt.Errorf("client is nil")
	}
	dm := session.DomainManager()

	ccDomain, err := dm.GetSharedDomain(d.Id())
	if err == nil {
		domainParts := strings.Split(ccDomain.Name, ".")
		subDomain := domainParts[0]
		domain := strings.Join(domainParts[1:], ".")

		d.Set("name", ccDomain.Name)
		d.Set("sub_domain", subDomain)
		d.Set("domain", domain)
		d.Set("route_group", ccDomain.RouterGroupGUID)
		d.Set("router_type", ccDomain.RouterType)

		return nil, err
	}

	ccDomain, err = dm.GetPrivateDomain(d.Id())
	if err == nil {
		domainParts := strings.Split(ccDomain.Name, ".")
		subDomain := domainParts[0]
		domain := strings.Join(domainParts[1:], ".")

		d.Set("name", ccDomain.Name)
		d.Set("sub_domain", subDomain)
		d.Set("domain", domain)
		d.Set("org", ccDomain.OwningOrganizationGUID)

		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}
