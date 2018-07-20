package cloudfoundry

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
)

const importStateKey = "is_import_state"

// getListOfStructs
func getListOfStructs(v interface{}) []map[string]interface{} {
	vvv := []map[string]interface{}{}
	for _, vv := range v.([]interface{}) {
		vvv = append(vvv, vv.(map[string]interface{}))
	}
	return vvv
}

// getChangedValueString
func getChangedValueString(key string, updated *bool, d *schema.ResourceData) *string {

	if d.HasChange(key) {
		vv := d.Get(key).(string)
		*updated = *updated || true
		return &vv
	} else if v, ok := d.GetOk(key); ok {
		vv := v.(string)
		return &vv
	}
	return nil
}

// getChangedValueInt
func getChangedValueInt(key string, updated *bool, d *schema.ResourceData) *int {

	if d.HasChange(key) {
		vv := d.Get(key).(int)
		*updated = *updated || true
		return &vv
	} else if v, ok := d.GetOk(key); ok {
		vv := v.(int)
		return &vv
	}
	return nil
}

// getChangedValueBool
func getChangedValueBool(key string, updated *bool, d *schema.ResourceData) *bool {

	if d.HasChange(key) {
		vv := d.Get(key).(bool)
		*updated = *updated || true
		return &vv
	} else if v, ok := d.GetOk(key); ok {
		vv := v.(bool)
		return &vv
	}
	return nil
}

// getChangedValueIntList
func getChangedValueIntList(key string, updated *bool, d *schema.ResourceData) *[]int {

	var a []interface{}

	if d.HasChange(key) {
		a = d.Get(key).(*schema.Set).List()
		*updated = *updated || true
	} else if v, ok := d.GetOk(key); ok {
		a = v.(*schema.Set).List()
	}
	if a != nil {
		aa := []int{}
		for _, vv := range a {
			aa = append(aa, vv.(int))
		}
		return &aa
	}
	return nil
}

// getChangedValueMap -
func getChangedValueMap(key string, updated *bool, d *schema.ResourceData) *map[string]interface{} {

	if d.HasChange(key) {
		vv := d.Get(key).(map[string]interface{})
		*updated = *updated || true
		return &vv
	} else if v, ok := d.GetOk(key); ok {
		vv := v.(map[string]interface{})
		return &vv
	}
	return nil
}

// getResourceChange -
func getResourceChange(key string, d *schema.ResourceData) (bool, string, string) {
	old, new := d.GetChange(key)
	return old != new, old.(string), new.(string)
}

// getListChanges -
func getListChanges(old interface{}, new interface{}) (remove []string, add []string) {

	var a bool

	for _, o := range old.(*schema.Set).List() {
		remove = append(remove, o.(string))
	}
	for _, n := range new.(*schema.Set).List() {
		nn := n.(string)
		a = true
		for i, r := range remove {
			if nn == r {
				remove = append(remove[:i], remove[i+1:]...)
				a = false
				break
			}
		}
		if a {
			add = append(add, nn)
		}
	}
	return remove, add
}

// getListChangedSchemaLists -
func getListChangedSchemaLists(old []interface{}, new []interface{}) (remove []map[string]interface{}, add []map[string]interface{}) {

	var a bool

	for _, o := range old {
		remove = append(remove, o.(map[string]interface{}))
	}
	for _, n := range new {
		nn := n.(map[string]interface{})
		a = true
		for i, r := range remove {
			if reflect.DeepEqual(nn, r) {
				remove = append(remove[:i], remove[i+1:]...)
				a = false
				break
			}
		}
		if a {
			add = append(add, nn)
		}
	}
	return remove, add
}

// ImportStatePassthrough -
func ImportStatePassthrough(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	MarkImportState(d)
	return schema.ImportStatePassthrough(d, meta)
}

// MarkImportState -
func MarkImportState(d *schema.ResourceData) {
	connInfo := d.ConnInfo()
	if connInfo == nil {
		connInfo = make(map[string]string)
	}
	connInfo[importStateKey] = ""
	d.SetConnInfo(connInfo)
}

// IsImportState -
func IsImportState(d *schema.ResourceData) bool {
	connInfo := d.ConnInfo()
	if connInfo == nil {
		return false
	}
	_, ok := connInfo[importStateKey]
	return ok
}

func computeID(first, second string) string {
	return fmt.Sprintf("%s/%s", first, second)
}

func parseID(id string) (first string, second string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		err = fmt.Errorf("unable to parse ID '%s', expected format is '<guid>/<guid>'", id)
	} else {
		first = parts[0]
		second = parts[1]
	}
	return first, second, err
}

func hashRouteMappingSet(v interface{}) int {
	elem := v.(map[string]interface{})
	var target string
	if v, ok := elem["route"]; ok {
		target = v.(string)
	} else if v, ok := elem["app"]; ok {
		target = v.(string)
	}
	return hashcode.String(fmt.Sprintf("%s", target))
}
