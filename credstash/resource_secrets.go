package credstash

import (
	"fmt"
	"log"

	"github.com/Clever/unicreds"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceCredstashSecret() *schema.Resource {

	return &schema.Resource{
		Create: resourceSecretPut,
		Read:   resourceSecretRead,
		Update: resourceSecretPut,
		Delete: resourceSecretDelete,
		Exists: resourceSecretExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the secret",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Value of the secret",
			},
			"context": {
				Type:        schema.TypeMap,
				Optional:    true,
				Description: "Encryption context for the secret",
			},
			"overwrite": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Version of the secrets",
			},
		},
	}
}

func resourceSecretExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	config := meta.(*Config)
	name := d.Id()
	log.Printf("[DEBUG] Checking secret name=%q", name)
	_, err := unicreds.GetHighestVersion(&config.TableName, name)
	if err != nil {
		if err == unicreds.ErrSecretNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func resourceSecretPut(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	name := d.Get("name").(string)
	log.Printf("[DEBUG] Writing secret name=%q", name)
	value := d.Get("value").(string)

	version, err := unicreds.ResolveVersion(&config.TableName, name, 0)
	if err != nil {
		return err
	}

	if shouldUpdateSecret(d) {
		context := getContext(d)
		log.Printf("[DEBUG] Writing secret for name=%q version=%q context=%+v", name, version, context)
		err = unicreds.PutSecret(&config.TableName, config.KmsKey, name, value, version, context)
		if err != nil {
			return err
		}
	}

	d.SetId(getID(d))
	return resourceSecretRead(d, meta)
}

func resourceSecretRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	name := d.Id()
	log.Printf("[DEBUG] Reading secret name=%q", name)
	version := d.Get("version").(string)

	if version == "" {
		v, err := unicreds.GetHighestVersion(&config.TableName, name)
		if err != nil {
			return err
		}
		version = v
	}

	context := getContext(d)
	log.Printf("[DEBUG] Getting secret for name=%q version=%q context=%+v", name, version, context)
	out, err := unicreds.GetSecret(&config.TableName, name, version, context)
	if err != nil {
		return err
	}

	d.Set("value", out.Secret)
	d.Set("name", name)
	d.Set("version", version)

	return nil
}

func resourceSecretDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	name := d.Id()

	err := unicreds.DeleteSecret(&config.TableName, name)
	if err != nil {
		return err
	}
	return nil
}

func getContext(d *schema.ResourceData) *unicreds.EncryptionContextValue {
	context := unicreds.NewEncryptionContextValue()
	for k, v := range d.Get("context").(map[string]interface{}) {
		context.Set(fmt.Sprintf("%s:%v", k, v))
	}
	return context
}

func shouldUpdateSecret(d *schema.ResourceData) bool {
	// if it is a new resource
	if d.IsNewResource() {
		return true
	}

	// If the user has specified a preference, return their preference
	if value, ok := d.GetOkExists("overwrite"); ok {
		return value.(bool)
	}

	// Since the user has not specified a preference, obey lifecycle rules
	return false
}
