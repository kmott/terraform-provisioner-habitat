package habitat

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/hashicorp/terraform/configs/hcl2shim"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-linereader"
)

type provisioner struct {
	Version          string
	License          string
	AutoUpdate       bool
	HttpDisable      bool
	Services         []Service
	PermanentPeer    bool
	ListenCtl        string
	ListenGossip     string
	ListenHTTP       string
	Peers            []string
	RingKey          string
	RingKeyContent   string
	CtlSecret        string
	SkipInstall      bool
	UseSudo          bool
	ServiceType      string
	ServiceName      string
	URL              string
	Channel          string
	Events           string
	Organization     string
	GatewayAuthToken string
	BuilderAuthToken string
	EventStream      *EventStream
	SupOptions       string

	installHabitat        provisionFn
	startHabitat          provisionFn
	uploadRingKey         provisionFn
	uploadCtlSecret       provisionFn
	uploadServiceGroupKey provisionServiceFn
	startHabitatService   provisionServiceFn

	osType string
}

type provisionFn func(terraform.UIOutput, communicator.Communicator) error
type provisionServiceFn func(terraform.UIOutput, communicator.Communicator, Service) error

func Provision() terraform.ResourceProvisioner {
	return &schema.Provisioner{
		Schema: map[string]*schema.Schema{
			"version": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "latest",
			},
			"license": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"auto_update": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"http_disable": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"peers": &schema.Schema{
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
			"service_type": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "systemd",
				ValidateFunc: validation.StringInSlice([]string{"systemd", "unmanaged"}, false),
			},
			"service_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "hab-supervisor",
			},
			"use_sudo": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"permanent_peer": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"listen_ctl": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"listen_gossip": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"listen_http": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"ring_key": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"ring_key_content": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"ctl_secret": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"url": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					u, err := url.Parse(val.(string))
					if err != nil {
						errs = append(errs, fmt.Errorf("invalid URL specified for %q: %v", key, err))
					}

					if u.Scheme == "" {
						errs = append(errs, fmt.Errorf("invalid URL specified for %q (scheme must be specified)", key))
					}

					return warns, errs
				},
			},
			"channel": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"events": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"organization": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"gateway_auth_token": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"builder_auth_token": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"event_stream": &schema.Schema{
				Type:     schema.TypeSet,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"application": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"environment": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"connect_timeout": &schema.Schema{
							Type:     schema.TypeInt,
							Optional: true,
						},
						"meta": &schema.Schema{
							Type:     schema.TypeMap,
							Optional: true,
						},
						"server_certificate": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"site": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"token": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"url": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
								u, err := url.Parse(val.(string))
								if err != nil {
									errs = append(errs, fmt.Errorf("invalid URL specified for \"event_stream.%s\": %v", key, err))
								}

								if u.Scheme == "" {
									errs = append(errs, fmt.Errorf("invalid URL specified for \"event_stream.%s\" (scheme must be specified)", key))
								}

								return warns, errs
							},
						},
					},
				},
				Optional: true,
			},
			"service": &schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"binds": &schema.Schema{
							Type:     schema.TypeList,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Optional: true,
						},
						"bind": &schema.Schema{
							Type: schema.TypeSet,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"alias": &schema.Schema{
										Type:     schema.TypeString,
										Required: true,
									},
									"service": &schema.Schema{
										Type:     schema.TypeString,
										Required: true,
									},
									"group": &schema.Schema{
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
							Optional: true,
						},
						"topology": &schema.Schema{
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice([]string{"leader", "standalone"}, false),
						},
						"strategy": &schema.Schema{
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice([]string{"none", "rolling", "at-once"}, false),
						},
						"user_toml": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"channel": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"group": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"url": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
								u, err := url.Parse(val.(string))
								if err != nil {
									errs = append(errs, fmt.Errorf("invalid URL specified for %q: %v", key, err))
								}

								if u.Scheme == "" {
									errs = append(errs, fmt.Errorf("invalid URL specified for %q (scheme must be specified)", key))
								}

								return warns, errs
							},
						},
						"application": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"environment": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"service_key": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
						"reprovision": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
				Optional: true,
			},
		},
		ApplyFunc:    applyFn,
		ValidateFunc: validateFn,
	}
}

func applyFn(ctx context.Context) error {
	o := ctx.Value(schema.ProvOutputKey).(terraform.UIOutput)
	s := ctx.Value(schema.ProvRawStateKey).(*terraform.InstanceState)
	d := ctx.Value(schema.ProvConfigDataKey).(*schema.ResourceData)

	p, err := decodeConfig(d)
	if err != nil {
		return err
	}

	// Automatically determine the OS type
	switch t := s.Ephemeral.ConnInfo["type"]; t {
	case "ssh", "":
		p.osType = "linux"
	case "winrm":
		p.osType = "windows"
	default:
		return fmt.Errorf("unsupported connection type: %s", t)
	}

	switch p.osType {
	case "linux":
		p.installHabitat = p.linuxInstallHabitat
		p.uploadRingKey = p.linuxUploadRingKey
		p.uploadCtlSecret = p.linuxUploadCtlSecret
		p.uploadServiceGroupKey = p.linuxUploadServiceGroupKey
		p.startHabitat = p.linuxStartHabitat
		p.startHabitatService = p.linuxStartHabitatService
	case "windows":
		p.installHabitat = p.windowsInstallHabitat
		p.uploadRingKey = p.windowsUploadRingKey
		p.uploadCtlSecret = p.windowsUploadCtlSecret
		p.uploadServiceGroupKey = p.windowsUploadServiceGroupKey
		p.startHabitat = p.windowsStartHabitat
		p.startHabitatService = p.windowsStartHabitatService
	default:
		return fmt.Errorf("unsupported os type: %s", p.osType)
	}

	// Get a new communicator
	comm, err := communicator.New(s)
	if err != nil {
		return err
	}

	retryCtx, cancel := context.WithTimeout(ctx, comm.Timeout())
	defer cancel()

	// Wait and retry until we establish the connection
	err = communicator.Retry(retryCtx, func() error {
		return comm.Connect(o)
	})

	if err != nil {
		return err
	}
	defer comm.Disconnect() //nolint:errcheck

	if !p.SkipInstall {
		o.Output("Installing habitat...")
		if err := p.installHabitat(o, comm); err != nil {
			return err
		}
	}

	if p.RingKeyContent != "" {
		o.Output("Uploading supervisor ring key...")
		if err := p.uploadRingKey(o, comm); err != nil {
			return err
		}
	}

	if p.CtlSecret != "" {
		o.Output("Uploading ctl secret...")
		if err := p.uploadCtlSecret(o, comm); err != nil {
			return err
		}
	}

	o.Output("Starting the habitat supervisor...")
	if err := p.startHabitat(o, comm); err != nil {
		return err
	}

	if p.Services != nil {
		for _, service := range p.Services {
			o.Output("Starting service: " + service.Name)
			if err := p.startHabitatService(o, comm, service); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateFn(c *terraform.ResourceConfig) (ws []string, es []error) {
	// Validate main config opts
	ringKeyContent, ok := c.Get("ring_key_content")
	if ok && ringKeyContent != "" && ringKeyContent != hcl2shim.UnknownVariableValue {
		ringKey, ringOk := c.Get("ring_key")
		if ringOk && ringKey == "" {
			es = append(es, errors.New("if ring_key_content is specified, ring_key must be specified as well"))
		}
	}

	// Validate service level opts
	services, ok := c.Get("service")
	if ok {
		data, dataOk := services.(string)
		if dataOk {
			es = append(es, fmt.Errorf("service '%v': must be a block", data))
		}
	}

	// Validate event stream opts
	eventStream, ok := c.Get("event_stream")
	if ok {
		data, dataOk := eventStream.(string)
		if dataOk {
			es = append(es, fmt.Errorf("event_stream '%v': must be a block", data))
		}
	}

	return ws, es
}

type Service struct {
	Name            string
	Strategy        string
	Topology        string
	Channel         string
	Group           string
	URL             string
	Binds           []Bind
	BindStrings     []string
	UserTOML        string
	AppName         string
	Environment     string
	ServiceGroupKey string
	Reprovision     bool
}

func (s *Service) getPackageName(fullName string) string {
	return strings.Split(fullName, "/")[1]
}

func (s *Service) getServiceNameChecksum() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s.Name)))
}

type Bind struct {
	Alias   string
	Service string
	Group   string
}

func (b *Bind) toBindString() string {
	return fmt.Sprintf("%s:%s.%s", b.Alias, b.Service, b.Group)
}

func decodeConfig(d *schema.ResourceData) (*provisioner, error) {
	p := &provisioner{
		Version:          d.Get("version").(string),
		License:          d.Get("license").(string),
		AutoUpdate:       d.Get("auto_update").(bool),
		HttpDisable:      d.Get("http_disable").(bool),
		Peers:            getPeers(d.Get("peers").([]interface{})),
		Services:         getServices(d.Get("service").(*schema.Set).List()),
		UseSudo:          d.Get("use_sudo").(bool),
		ServiceType:      d.Get("service_type").(string),
		ServiceName:      d.Get("service_name").(string),
		RingKey:          d.Get("ring_key").(string),
		RingKeyContent:   d.Get("ring_key_content").(string),
		CtlSecret:        d.Get("ctl_secret").(string),
		PermanentPeer:    d.Get("permanent_peer").(bool),
		ListenCtl:        d.Get("listen_ctl").(string),
		ListenGossip:     d.Get("listen_gossip").(string),
		ListenHTTP:       d.Get("listen_http").(string),
		URL:              d.Get("url").(string),
		Channel:          d.Get("channel").(string),
		Events:           d.Get("events").(string),
		Organization:     d.Get("organization").(string),
		BuilderAuthToken: d.Get("builder_auth_token").(string),
		GatewayAuthToken: d.Get("gateway_auth_token").(string),
		EventStream:      getEventStream(d.Get("event_stream").(*schema.Set).List()),
	}

	return p, nil
}

func getPeers(v []interface{}) []string {
	peers := make([]string, 0, len(v))
	for _, rawPeerData := range v {
		peers = append(peers, rawPeerData.(string))
	}
	return peers
}

func getServices(v []interface{}) []Service {
	services := make([]Service, 0, len(v))
	for _, rawServiceData := range v {
		serviceData := rawServiceData.(map[string]interface{})
		name := serviceData["name"].(string)
		strategy := serviceData["strategy"].(string)
		topology := serviceData["topology"].(string)
		channel := serviceData["channel"].(string)
		group := serviceData["group"].(string)
		url := serviceData["url"].(string)
		app := serviceData["application"].(string)
		env := serviceData["environment"].(string)
		userToml := serviceData["user_toml"].(string)
		serviceGroupKey := serviceData["service_key"].(string)
		reprovision := serviceData["reprovision"].(bool)
		var bindStrings []string
		binds := getBinds(serviceData["bind"].(*schema.Set).List())
		for _, b := range serviceData["binds"].([]interface{}) {
			bind, err := getBindFromString(b.(string))
			if err != nil {
				return nil
			}
			binds = append(binds, bind)
		}

		service := Service{
			Name:            name,
			Strategy:        strategy,
			Topology:        topology,
			Channel:         channel,
			Group:           group,
			URL:             url,
			UserTOML:        userToml,
			BindStrings:     bindStrings,
			Binds:           binds,
			AppName:         app,
			Environment:     env,
			ServiceGroupKey: serviceGroupKey,
			Reprovision:     reprovision,
		}
		services = append(services, service)
	}
	return services
}

func getBinds(v []interface{}) []Bind {
	binds := make([]Bind, 0, len(v))
	for _, rawBindData := range v {
		bindData := rawBindData.(map[string]interface{})
		alias := bindData["alias"].(string)
		service := bindData["service"].(string)
		group := bindData["group"].(string)
		bind := Bind{
			Alias:   alias,
			Service: service,
			Group:   group,
		}
		binds = append(binds, bind)
	}
	return binds
}

func (p *provisioner) copyOutput(o terraform.UIOutput, r io.Reader) {
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}

func (p *provisioner) runCommand(o terraform.UIOutput, comm communicator.Communicator, command string) error {
	outR, outW := io.Pipe()
	errR, errW := io.Pipe()

	go p.copyOutput(o, outR)
	go p.copyOutput(o, errR)
	defer outW.Close()
	defer errW.Close()

	cmd := &remote.Cmd{
		Command: command,
		Stdout:  outW,
		Stderr:  errW,
	}

	if err := comm.Start(cmd); err != nil {
		return fmt.Errorf("error executing command %q: %v", cmd.Command, err)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func getBindFromString(bind string) (Bind, error) {
	t := strings.FieldsFunc(bind, func(d rune) bool {
		switch d {
		case ':', '.':
			return true
		}
		return false
	})
	if len(t) != 3 {
		return Bind{}, errors.New("invalid bind specification: " + bind)
	}
	return Bind{Alias: t[0], Service: t[1], Group: t[2]}, nil
}

type EventStream struct {
	Application       string
	Environment       string
	ConnectTimeout    int
	Meta              map[string]interface{}
	ServerCertificate string
	Site              string
	Token             string
	Url               string
}

func (es *EventStream) FlagValues() string {
	var flags string

	if es.Application != "" {
		flags += fmt.Sprintf(" --event-stream-application %s", es.Application)
	}

	if es.Environment != "" {
		flags += fmt.Sprintf(" --event-stream-environment %s", es.Environment)
	}

	flags += fmt.Sprintf(" --event-stream-connect-timeout %v", es.ConnectTimeout)

	metaTags := es.getSortedMetaTags()
	if len(metaTags) > 0 {
		flags += fmt.Sprintf(" --event-meta %q", strings.Join(metaTags, " "))
	}

	if es.ServerCertificate != "" {
		flags += fmt.Sprintf(" --event-stream-server-certificate %s", es.ServerCertificate)
	}

	if es.Site != "" {
		flags += fmt.Sprintf(" --event-stream-site %s", es.Site)
	}

	if es.Token != "" {
		flags += fmt.Sprintf(" --event-stream-token %s", es.Token)
	}

	if es.Url != "" {
		flags += fmt.Sprintf(" --event-stream-url %s", es.Url)
	}

	return flags
}

// Pulled this out to it's own method so we can sort the meta tag by keys for more consistent testing
func (es *EventStream) getSortedMetaTags() []string {
	// This stores the actual meta tags
	var tags []string

	// Pull out the keys so we can sort them using sort.Strings()
	keys := make([]string, 0, len(es.Meta))
	for k := range es.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Range over the sorted keys now, and add them to our tags[] slice
	for _, k := range keys {
		tags = append(tags, fmt.Sprintf("%s=%s", k, es.Meta[k]))
	}

	return tags
}

func getEventStream(v []interface{}) *EventStream {
	if len(v) > 0 {
		es := &EventStream{}
		for _, rawEventStreamData := range v {
			eventStreamData := rawEventStreamData.(map[string]interface{})
			es.Application = eventStreamData["application"].(string)
			es.Environment = eventStreamData["environment"].(string)
			es.ConnectTimeout = eventStreamData["connect_timeout"].(int)
			es.Meta = eventStreamData["meta"].(map[string]interface{})
			es.ServerCertificate = eventStreamData["server_certificate"].(string)
			es.Site = eventStreamData["site"].(string)
			es.Token = eventStreamData["token"].(string)
			es.Url = eventStreamData["url"].(string)
		}
		return es
	}

	return nil
}
