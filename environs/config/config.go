// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/schema"
	"github.com/juju/utils"
	"github.com/juju/utils/proxy"
	"github.com/juju/utils/series"
	"github.com/juju/version"
	"gopkg.in/juju/charmrepo.v2"
	"gopkg.in/juju/environschema.v1"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/controller"
	"github.com/juju/juju/environs/tags"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/logfwd/syslog"
	"github.com/juju/juju/network"
)

var logger = loggo.GetLogger("juju.environs.config")

const (
	// FwInstance requests the use of an individual firewall per instance.
	FwInstance = "instance"

	// FwGlobal requests the use of a single firewall group for all machines.
	// When ports are opened for one machine, all machines will have the same
	// port opened.
	FwGlobal = "global"

	// FwNone requests that no firewalling should be performed inside
	// the environment. No firewaller worker will be started. It's
	// useful for clouds without support for either global or per
	// instance security groups.
	FwNone = "none"
)

// TODO(katco-): Please grow this over time.
// Centralized place to store values of config keys. This transitions
// mistakes in referencing key-values to a compile-time error.
const (
	//
	// Settings Attributes
	//

	// NameKey is the key for the model's name.
	NameKey = "name"

	// TypeKey is the key for the model's cloud type.
	TypeKey = "type"

	// AgentVersionKey is the key for the model's Juju agent version.
	AgentVersionKey = "agent-version"

	// UUIDKey is the key for the model UUID attribute.
	UUIDKey = "uuid"

	// AuthorizedKeysKey is the key for the authorized-keys attribute.
	AuthorizedKeysKey = "authorized-keys"

	// ProvisionerHarvestModeKey stores the key for this setting.
	ProvisionerHarvestModeKey = "provisioner-harvest-mode"

	// AgentStreamKey stores the key for this setting.
	AgentStreamKey = "agent-stream"

	// AgentMetadataURLKey stores the key for this setting.
	AgentMetadataURLKey = "agent-metadata-url"

	// HTTPProxyKey stores the key for this setting.
	HTTPProxyKey = "http-proxy"

	// HTTPSProxyKey stores the key for this setting.
	HTTPSProxyKey = "https-proxy"

	// FTPProxyKey stores the key for this setting.
	FTPProxyKey = "ftp-proxy"

	// NoProxyKey stores the key for this setting.
	NoProxyKey = "no-proxy"

	// AptHTTPProxyKey stores the key for this setting.
	AptHTTPProxyKey = "apt-http-proxy"

	// AptHTTPSProxyKey stores the key for this setting.
	AptHTTPSProxyKey = "apt-https-proxy"

	// AptFTPProxyKey stores the key for this setting.
	AptFTPProxyKey = "apt-ftp-proxy"

	// AptNoProxyKey stores the key for this setting.
	AptNoProxyKey = "apt-no-proxy"

	// NetBondReconfigureDelay is the key to pass when bridging
	// the network for containers.
	NetBondReconfigureDelayKey = "net-bond-reconfigure-delay"

	// ContainerNetworkingMethod is the key for setting up
	// networking method for containers.
	ContainerNetworkingMethod = "container-networking-method"

	// The default block storage source.
	StorageDefaultBlockSourceKey = "storage-default-block-source"

	// The default filesystem storage source.
	StorageDefaultFilesystemSourceKey = "storage-default-filesystem-source"

	// ResourceTagsKey is an optional list or space-separated string
	// of k=v pairs, defining the tags for ResourceTags.
	ResourceTagsKey = "resource-tags"

	// LogForwardEnabled determines whether the log forward functionality is enabled.
	LogForwardEnabled = "logforward-enabled"

	// LogFwdSyslogHost sets the hostname:port of the syslog server.
	LogFwdSyslogHost = "syslog-host"

	// LogFwdSyslogCACert sets the certificate of the CA that signed the syslog
	// server certificate.
	LogFwdSyslogCACert = "syslog-ca-cert"

	// LogFwdSyslogClientCert sets the client certificate for syslog
	// forwarding.
	LogFwdSyslogClientCert = "syslog-client-cert"

	// LogFwdSyslogClientKey sets the client key for syslog
	// forwarding.
	LogFwdSyslogClientKey = "syslog-client-key"

	// AutomaticallyRetryHooks determines whether the uniter will
	// automatically retry a hook that has failed
	AutomaticallyRetryHooks = "automatically-retry-hooks"

	// TransmitVendorMetricsKey is the key for whether the controller sends
	// metrics collected in this model for anonymized aggregate analytics.
	TransmitVendorMetricsKey = "transmit-vendor-metrics"

	// ExtraInfoKey is the key for arbitrary user specified string data that
	// is stored against the model.
	ExtraInfoKey = "extra-info"

	// MaxStatusHistoryAge is the maximum age of status history values
	// to keep when pruning, eg "72h"
	MaxStatusHistoryAge = "max-status-history-age"

	// MaxStatusHistorySize is the maximum size the status history
	// collection can grow to before it is pruned, eg "5M"
	MaxStatusHistorySize = "max-status-history-size"

	// MaxActionResultsAge is the maximum age of actions to keep when pruning, eg
	// "72h"
	MaxActionResultsAge = "max-action-results-age"

	// MaxActionResultsSize is the maximum size the actions collection can
	// grow to before it is pruned, eg "5M"
	MaxActionResultsSize = "max-action-results-size"

	// UpdateStatusHookInterval is how often to run the update-status hook.
	UpdateStatusHookInterval = "update-status-hook-interval"

	// EgressSubnets are the source addresses from which traffic from this model
	// originates if the model is deployed such that NAT or similar is in use.
	EgressSubnets = "egress-subnets"

	// FanConfig defines the configuration for FAN network running in the model.
	FanConfig = "fan-config"

	//
	// Deprecated Settings Attributes
	//

	// IgnoreMachineAddresses, when true, will cause the
	// machine worker not to discover any machine addresses
	// on start up.
	IgnoreMachineAddresses = "ignore-machine-addresses"
)

// ParseHarvestMode parses description of harvesting method and
// returns the representation.
func ParseHarvestMode(description string) (HarvestMode, error) {
	description = strings.ToLower(description)
	for method, descr := range harvestingMethodToFlag {
		if description == descr {
			return method, nil
		}
	}
	return 0, fmt.Errorf("unknown harvesting method: %s", description)
}

// HarvestMode is a bit field which is used to store the harvesting
// behavior for Juju.
type HarvestMode uint32

const (
	// HarvestNone signifies that Juju should not harvest any
	// machines.
	HarvestNone HarvestMode = 1 << iota
	// HarvestUnknown signifies that Juju should only harvest machines
	// which exist, but we don't know about.
	HarvestUnknown
	// HarvestDestroyed signifies that Juju should only harvest
	// machines which have been explicitly released by the user
	// through a destroy of a service/model/unit.
	HarvestDestroyed
	// HarvestAll signifies that Juju should harvest both unknown and
	// destroyed instances. ♫ Don't fear the reaper. ♫
	HarvestAll HarvestMode = HarvestUnknown | HarvestDestroyed
)

// A mapping from method to description. Going this way will be the
// more common operation, so we want this type of lookup to be O(1).
var harvestingMethodToFlag = map[HarvestMode]string{
	HarvestAll:       "all",
	HarvestNone:      "none",
	HarvestUnknown:   "unknown",
	HarvestDestroyed: "destroyed",
}

// String returns the description of the harvesting mode.
func (method HarvestMode) String() string {
	if description, ok := harvestingMethodToFlag[method]; ok {
		return description
	}
	panic("Unknown harvesting method.")
}

// None returns whether or not the None harvesting flag is set.
func (method HarvestMode) HarvestNone() bool {
	return method&HarvestNone != 0
}

// Destroyed returns whether or not the Destroyed harvesting flag is set.
func (method HarvestMode) HarvestDestroyed() bool {
	return method&HarvestDestroyed != 0
}

// Unknown returns whether or not the Unknown harvesting flag is set.
func (method HarvestMode) HarvestUnknown() bool {
	return method&HarvestUnknown != 0
}

type HasDefaultSeries interface {
	DefaultSeries() (string, bool)
}

// PreferredSeries returns the preferred series to use when a charm does not
// explicitly specify a series.
func PreferredSeries(cfg HasDefaultSeries) string {
	if series, ok := cfg.DefaultSeries(); ok {
		return series
	}
	return series.LatestLts()
}

// Config holds an immutable environment configuration.
type Config struct {
	// defined holds the attributes that are defined for Config.
	// unknown holds the other attributes that are passed in (aka UnknownAttrs).
	// the union of these two are AllAttrs
	defined, unknown map[string]interface{}
}

// Defaulting is a value that specifies whether a configuration
// creator should use defaults from the environment.
type Defaulting bool

const (
	UseDefaults Defaulting = true
	NoDefaults  Defaulting = false
)

// TODO(rog) update the doc comment below - it's getting messy
// and it assumes too much prior knowledge.

// New returns a new configuration.  Fields that are common to all
// environment providers are verified.  If useDefaults is UseDefaults,
// default values will be taken from the environment.
//
// "ca-cert-path" and "ca-private-key-path" are translated into the
// "ca-cert" and "ca-private-key" values.  If not specified, CA details
// will be read from:
//
//     ~/.local/share/juju/<name>-cert.pem
//     ~/.local/share/juju/<name>-private-key.pem
//
// if $XDG_DATA_HOME is defined it will be used instead of ~/.local/share
func New(withDefaults Defaulting, attrs map[string]interface{}) (*Config, error) {
	checker := noDefaultsChecker
	if withDefaults {
		checker = withDefaultsChecker
	}
	defined, err := checker.Coerce(attrs, nil)
	if err != nil {
		return nil, err
	}

	c := &Config{
		defined: defined.(map[string]interface{}),
		unknown: make(map[string]interface{}),
	}
	if err := c.ensureUnitLogging(); err != nil {
		return nil, err
	}

	// no old config to compare against
	if err := Validate(c, nil); err != nil {
		return nil, err
	}
	// Copy unknown attributes onto the type-specific map.
	for k, v := range attrs {
		if _, ok := fields[k]; !ok {
			c.unknown[k] = v
		}
	}
	return c, nil
}

const (
	// DefaultStatusHistoryAge is the default value for MaxStatusHistoryAge.
	DefaultStatusHistoryAge = "336h" // 2 weeks

	// DefaultStatusHistorySize is the default value for MaxStatusHistorySize.
	DefaultStatusHistorySize = "5G"

	// DefaultUpdateStatusHookInterval is the default value for UpdateStatusHookInterval
	DefaultUpdateStatusHookInterval = "5m"

	DefaultActionResultsAge = "336h" // 2 weeks

	DefaultActionResultsSize = "5G"
)

var defaultConfigValues = map[string]interface{}{
	// Network.
	"firewall-mode":              FwInstance,
	"disable-network-management": false,
	IgnoreMachineAddresses:       false,
	"ssl-hostname-verification":  true,
	"proxy-ssh":                  false,

	// Why is net-bond-reconfigure-delay set to 17 seconds?
	//
	// The value represents the amount of time in seconds to sleep
	// between ifdown and ifup when bridging bonded interfaces;
	// this is a platform bug and all of this can go away when bug
	// #1657579 (and #1594855 and #1269921) are fixed.
	//
	// For a long time the bridge script hardcoded a value of 3s
	// but some setups now require an even longer period. The last
	// reported issue was fixed with a 10s timeout, however, we're
	// increasing that because this issue (and solution) is not
	// very discoverable and we would like bridging to work
	// out-of-the-box.
	//
	// This value can be further tweaked via:
	//
	// $ juju model-config net-bond-reconfigure-delay=30
	NetBondReconfigureDelayKey: 17,
	ContainerNetworkingMethod:  "",

	"default-series":           series.LatestLts(),
	ProvisionerHarvestModeKey:  HarvestDestroyed.String(),
	ResourceTagsKey:            "",
	"logging-config":           "",
	AutomaticallyRetryHooks:    true,
	"enable-os-refresh-update": true,
	"enable-os-upgrade":        true,
	"development":              false,
	"test-mode":                false,
	TransmitVendorMetricsKey:   true,
	UpdateStatusHookInterval:   DefaultUpdateStatusHookInterval,
	EgressSubnets:              "",
	FanConfig:                  "",

	// Image and agent streams and URLs.
	"image-stream":       "released",
	"image-metadata-url": "",
	AgentStreamKey:       "released",
	AgentMetadataURLKey:  "",

	// Log forward settings.
	LogForwardEnabled: false,

	// Proxy settings.
	HTTPProxyKey:     "",
	HTTPSProxyKey:    "",
	FTPProxyKey:      "",
	NoProxyKey:       "127.0.0.1,localhost,::1",
	AptHTTPProxyKey:  "",
	AptHTTPSProxyKey: "",
	AptFTPProxyKey:   "",
	AptNoProxyKey:    "",
	"apt-mirror":     "",

	// Status history settings
	MaxStatusHistoryAge:  DefaultStatusHistoryAge,
	MaxStatusHistorySize: DefaultStatusHistorySize,
	MaxActionResultsAge:  DefaultActionResultsAge,
	MaxActionResultsSize: DefaultActionResultsSize,
}

// ConfigDefaults returns the config default values
// to be used for any new model where there is no
// value yet defined.
func ConfigDefaults() map[string]interface{} {
	return defaultConfigValues
}

func (c *Config) ensureUnitLogging() error {
	loggingConfig := c.asString("logging-config")
	// If the logging config hasn't been set, then look for the os environment
	// variable, and failing that, get the config from loggo itself.
	if loggingConfig == "" {
		if environmentValue := os.Getenv(osenv.JujuLoggingConfigEnvKey); environmentValue != "" {
			loggingConfig = environmentValue
		} else {
			loggingConfig = loggo.LoggerInfo()
		}
	}
	levels, err := loggo.ParseConfigString(loggingConfig)
	if err != nil {
		return err
	}
	// If there is is no specified level for "unit", then set one.
	if _, ok := levels["unit"]; !ok {
		loggingConfig = loggingConfig + ";unit=DEBUG"
	}
	c.defined["logging-config"] = loggingConfig
	return nil
}

// ProcessDeprecatedAttributes gathers any deprecated attributes in attrs and adds or replaces
// them with new name value pairs for the replacement attrs.
// Ths ensures that older versions of Juju which require that deprecated
// attribute values still be used will work as expected.
func ProcessDeprecatedAttributes(attrs map[string]interface{}) map[string]interface{} {
	processedAttrs := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		processedAttrs[k] = v
	}
	// No deprecated attributes at the moment.
	return processedAttrs
}

// CoerceForStorage transforms attributes prior to being saved in a persistent store.
func CoerceForStorage(attrs map[string]interface{}) map[string]interface{} {
	coercedAttrs := make(map[string]interface{}, len(attrs))
	for attrName, attrValue := range attrs {
		if attrName == ResourceTagsKey {
			// Resource Tags are specified by the user as a string but transformed
			// to a map when config is parsed. We want to store as a string.
			var tagsSlice []string
			if tags, ok := attrValue.(map[string]string); ok {
				for resKey, resValue := range tags {
					tagsSlice = append(tagsSlice, fmt.Sprintf("%v=%v", resKey, resValue))
				}
				attrValue = strings.Join(tagsSlice, " ")
			}
		}
		coercedAttrs[attrName] = attrValue
	}
	return coercedAttrs
}

// Validate ensures that config is a valid configuration.  If old is not nil,
// it holds the previous environment configuration for consideration when
// validating changes.
func Validate(cfg, old *Config) error {
	// Check that all other fields that have been specified are non-empty,
	// unless they're allowed to be empty for backward compatibility,
	for attr, val := range cfg.defined {
		if !isEmpty(val) {
			continue
		}
		if !allowEmpty(attr) {
			return fmt.Errorf("empty %s in model configuration", attr)
		}
	}

	modelName := cfg.asString(NameKey)
	if modelName == "" {
		return errors.New("empty name in model configuration")
	}
	if !names.IsValidModelName(modelName) {
		return fmt.Errorf("%q is not a valid name: model names may only contain lowercase letters, digits and hyphens", modelName)
	}

	// Check that the agent version parses ok if set explicitly; otherwise leave
	// it alone.
	if v, ok := cfg.defined[AgentVersionKey].(string); ok {
		if _, err := version.Parse(v); err != nil {
			return fmt.Errorf("invalid agent version in model configuration: %q", v)
		}
	}

	// If the logging config is set, make sure it is valid.
	if v, ok := cfg.defined["logging-config"].(string); ok {
		if _, err := loggo.ParseConfigString(v); err != nil {
			return err
		}
	}

	if lfCfg, ok := cfg.LogFwdSyslog(); ok {
		if err := lfCfg.Validate(); err != nil {
			return errors.Annotate(err, "invalid syslog forwarding config")
		}
	}

	if uuid := cfg.UUID(); !utils.IsValidUUIDString(uuid) {
		return errors.Errorf("uuid: expected UUID, got string(%q)", uuid)
	}

	// Ensure the resource tags have the expected k=v format.
	if _, err := cfg.resourceTags(); err != nil {
		return errors.Annotate(err, "validating resource tags")
	}

	if v, ok := cfg.defined[MaxStatusHistoryAge].(string); ok {
		if _, err := time.ParseDuration(v); err != nil {
			return errors.Annotate(err, "invalid max status history age in model configuration")
		}
	}

	if v, ok := cfg.defined[MaxStatusHistorySize].(string); ok {
		if _, err := utils.ParseSize(v); err != nil {
			return errors.Annotate(err, "invalid max status history size in model configuration")
		}
	}

	if v, ok := cfg.defined[MaxActionResultsAge].(string); ok {
		if _, err := time.ParseDuration(v); err != nil {
			return errors.Annotate(err, "invalid max action age in model configuration")
		}
	}

	if v, ok := cfg.defined[MaxActionResultsSize].(string); ok {
		if _, err := utils.ParseSize(v); err != nil {
			return errors.Annotate(err, "invalid max action size in model configuration")
		}
	}

	if v, ok := cfg.defined[UpdateStatusHookInterval].(string); ok {
		if f, err := time.ParseDuration(v); err != nil {
			return errors.Annotate(err, "invalid update status hook interval in model configuration")
		} else {
			if f < 1*time.Minute {
				return errors.Annotatef(err, "update status hook frequency %v cannot be less than 1m", f)
			}
			if f > 60*time.Minute {
				return errors.Annotatef(err, "update status hook frequency %v cannot be greater than 60m", f)
			}
		}
	}

	if v, ok := cfg.defined[EgressSubnets].(string); ok && v != "" {
		cidrs := strings.Split(v, ",")
		for _, cidr := range cidrs {
			if _, _, err := net.ParseCIDR(strings.TrimSpace(cidr)); err != nil {
				return errors.Annotatef(err, "invalid egress subnet: %v", cidr)
			}
			if cidr == "0.0.0.0/0" {
				return errors.Errorf("CIDR %q not allowed", cidr)
			}
		}
	}

	if v, ok := cfg.defined[FanConfig].(string); ok && v != "" {
		_, err := network.ParseFanConfig(v)
		if err != nil {
			return err
		}
	}

	if v, ok := cfg.defined[ContainerNetworkingMethod].(string); ok {
		switch v {
		case "fan":
			if cfg, err := cfg.FanConfig(); err != nil || cfg == nil {
				return errors.New("container-networking-method cannot be set to 'fan' without fan-config set")
			}
		case "provider": // TODO(wpk) FIXME we should check that the provider supports this setting!
		case "local":
		case "": // We'll try to autoconfigure it
		default:
			return fmt.Errorf("Invalid value for container-networking-method - %v", v)
		}
	}
	// Check the immutable config values.  These can't change
	if old != nil {
		for _, attr := range immutableAttributes {
			oldv, ok := old.defined[attr]
			if !ok {
				continue
			}
			if newv := cfg.defined[attr]; newv != oldv {
				return fmt.Errorf("cannot change %s from %#v to %#v", attr, oldv, newv)
			}
		}
		if _, oldFound := old.AgentVersion(); oldFound {
			if _, newFound := cfg.AgentVersion(); !newFound {
				return errors.New("cannot clear agent-version")
			}
		}
	}

	cfg.defined = ProcessDeprecatedAttributes(cfg.defined)
	return nil
}

func isEmpty(val interface{}) bool {
	switch val := val.(type) {
	case nil:
		return true
	case bool:
		return false
	case int:
		// TODO(rog) fix this to return false when
		// we can lose backward compatibility.
		// https://bugs.launchpad.net/juju-core/+bug/1224492
		return val == 0
	case string:
		return val == ""
	case []interface{}:
		return len(val) == 0
	case map[string]string:
		return len(val) == 0
	}
	panic(fmt.Errorf("unexpected type %T in configuration", val))
}

// asString is a private helper method to keep the ugly string casting
// in once place. It returns the given named attribute as a string,
// returning "" if it isn't found.
func (c *Config) asString(name string) string {
	value, _ := c.defined[name].(string)
	return value
}

// mustString returns the named attribute as an string, panicking if
// it is not found or is empty.
func (c *Config) mustString(name string) string {
	value, _ := c.defined[name].(string)
	if value == "" {
		panic(fmt.Errorf("empty value for %q found in configuration (type %T, val %v)", name, c.defined[name], c.defined[name]))
	}
	return value
}

// Type returns the model's cloud provider type.
func (c *Config) Type() string {
	return c.mustString(TypeKey)
}

// Name returns the model name.
func (c *Config) Name() string {
	return c.mustString(NameKey)
}

// UUID returns the uuid for the model.
func (c *Config) UUID() string {
	return c.mustString(UUIDKey)
}

// DefaultSeries returns the configured default Ubuntu series for the environment,
// and whether the default series was explicitly configured on the environment.
func (c *Config) DefaultSeries() (string, bool) {
	s, ok := c.defined["default-series"]
	if !ok {
		return "", false
	}
	switch s := s.(type) {
	case string:
		return s, s != ""
	default:
		logger.Errorf("invalid default-series: %q", s)
		return "", false
	}
}

// AuthorizedKeys returns the content for ssh's authorized_keys file.
func (c *Config) AuthorizedKeys() string {
	value, _ := c.defined[AuthorizedKeysKey].(string)
	return value
}

// ProxySSH returns a flag indicating whether SSH commands
// should be proxied through the API server.
func (c *Config) ProxySSH() bool {
	value, _ := c.defined["proxy-ssh"].(bool)
	return value
}

// NetBondReconfigureDelay returns the duration in seconds that should be
// passed to the bridge script when bridging bonded interfaces.
func (c *Config) NetBondReconfigureDelay() int {
	value, _ := c.defined[NetBondReconfigureDelayKey].(int)
	return value
}

// ContainerNetworkingMethod returns the method with which
// containers network should be set up.
func (c *Config) ContainerNetworkingMethod() string {
	return c.asString(ContainerNetworkingMethod)
}

// ProxySettings returns all four proxy settings; http, https, ftp, and no
// proxy.
func (c *Config) ProxySettings() proxy.Settings {
	return proxy.Settings{
		Http:    c.HTTPProxy(),
		Https:   c.HTTPSProxy(),
		Ftp:     c.FTPProxy(),
		NoProxy: c.NoProxy(),
	}
}

// HTTPProxy returns the http proxy for the environment.
func (c *Config) HTTPProxy() string {
	return c.asString(HTTPProxyKey)
}

// HTTPSProxy returns the https proxy for the environment.
func (c *Config) HTTPSProxy() string {
	return c.asString(HTTPSProxyKey)
}

// FTPProxy returns the ftp proxy for the environment.
func (c *Config) FTPProxy() string {
	return c.asString(FTPProxyKey)
}

// NoProxy returns the 'no-proxy' for the environment.
func (c *Config) NoProxy() string {
	return c.asString(NoProxyKey)
}

func (c *Config) getWithFallback(key, fallback string) string {
	value := c.asString(key)
	if value == "" {
		value = c.asString(fallback)
	}
	return value
}

// addSchemeIfMissing adds a scheme to a URL if it is missing
func addSchemeIfMissing(defaultScheme string, url string) string {
	if url != "" && !strings.Contains(url, "://") {
		url = defaultScheme + "://" + url
	}
	return url
}

// AptProxySettings returns all three proxy settings; http, https and ftp.
func (c *Config) AptProxySettings() proxy.Settings {
	return proxy.Settings{
		Http:    c.AptHTTPProxy(),
		Https:   c.AptHTTPSProxy(),
		Ftp:     c.AptFTPProxy(),
		NoProxy: c.AptNoProxy(),
	}
}

// AptHTTPProxy returns the apt http proxy for the environment.
// Falls back to the default http-proxy if not specified.
func (c *Config) AptHTTPProxy() string {
	return addSchemeIfMissing("http", c.getWithFallback(AptHTTPProxyKey, HTTPProxyKey))
}

// AptHTTPSProxy returns the apt https proxy for the environment.
// Falls back to the default https-proxy if not specified.
func (c *Config) AptHTTPSProxy() string {
	return addSchemeIfMissing("https", c.getWithFallback(AptHTTPSProxyKey, HTTPSProxyKey))
}

// AptFTPProxy returns the apt ftp proxy for the environment.
// Falls back to the default ftp-proxy if not specified.
func (c *Config) AptFTPProxy() string {
	return addSchemeIfMissing("ftp", c.getWithFallback(AptFTPProxyKey, FTPProxyKey))
}

// AptNoProxy returns the 'apt-no-proxy' for the environment.
func (c *Config) AptNoProxy() string {
	return c.getWithFallback(AptNoProxyKey, NoProxyKey)
}

// AptMirror sets the apt mirror for the environment.
func (c *Config) AptMirror() string {
	return c.asString("apt-mirror")
}

// LogFwdSyslog returns the syslog forwarding config.
func (c *Config) LogFwdSyslog() (*syslog.RawConfig, bool) {
	partial := false
	var lfCfg syslog.RawConfig

	if s, ok := c.defined[LogForwardEnabled]; ok {
		partial = true
		lfCfg.Enabled = s.(bool)
	}

	if s, ok := c.defined[LogFwdSyslogHost]; ok && s != "" {
		partial = true
		lfCfg.Host = s.(string)
	}

	if s, ok := c.defined[LogFwdSyslogCACert]; ok && s != "" {
		partial = true
		lfCfg.CACert = s.(string)
	}

	if s, ok := c.defined[LogFwdSyslogClientCert]; ok && s != "" {
		partial = true
		lfCfg.ClientCert = s.(string)
	}

	if s, ok := c.defined[LogFwdSyslogClientKey]; ok && s != "" {
		partial = true
		lfCfg.ClientKey = s.(string)
	}

	if !partial {
		return nil, false
	}
	return &lfCfg, true
}

// FirewallMode returns whether the firewall should
// manage ports per machine, globally, or not at all.
// (FwInstance, FwGlobal, or FwNone).
func (c *Config) FirewallMode() string {
	return c.mustString("firewall-mode")
}

// AgentVersion returns the proposed version number for the agent tools,
// and whether it has been set. Once an environment is bootstrapped, this
// must always be valid.
func (c *Config) AgentVersion() (version.Number, bool) {
	if v, ok := c.defined[AgentVersionKey].(string); ok {
		n, err := version.Parse(v)
		if err != nil {
			panic(err) // We should have checked it earlier.
		}
		return n, true
	}
	return version.Zero, false
}

// AgentMetadataURL returns the URL that locates the agent tarballs and metadata,
// and whether it has been set.
func (c *Config) AgentMetadataURL() (string, bool) {
	if url, ok := c.defined[AgentMetadataURLKey]; ok && url != "" {
		return url.(string), true
	}
	return "", false
}

// ImageMetadataURL returns the URL at which the metadata used to locate image ids is located,
// and wether it has been set.
func (c *Config) ImageMetadataURL() (string, bool) {
	if url, ok := c.defined["image-metadata-url"]; ok && url != "" {
		return url.(string), true
	}
	return "", false
}

// Development returns whether the environment is in development mode.
func (c *Config) Development() bool {
	value, _ := c.defined["development"].(bool)
	return value
}

// EnableOSRefreshUpdate returns whether or not newly provisioned
// instances should run their respective OS's update capability.
func (c *Config) EnableOSRefreshUpdate() bool {
	if val, ok := c.defined["enable-os-refresh-update"].(bool); !ok {
		return true
	} else {
		return val
	}
}

// EnableOSUpgrade returns whether or not newly provisioned instances
// should run their respective OS's upgrade capability.
func (c *Config) EnableOSUpgrade() bool {
	if val, ok := c.defined["enable-os-upgrade"].(bool); !ok {
		return true
	} else {
		return val
	}
}

// SSLHostnameVerification returns weather the environment has requested
// SSL hostname verification to be enabled.
func (c *Config) SSLHostnameVerification() bool {
	return c.defined["ssl-hostname-verification"].(bool)
}

// LoggingConfig returns the configuration string for the loggers.
func (c *Config) LoggingConfig() string {
	return c.asString("logging-config")
}

// AutomaticallyRetryHooks returns whether we should automatically retry hooks.
// By default this should be true.
func (c *Config) AutomaticallyRetryHooks() bool {
	if val, ok := c.defined["automatically-retry-hooks"].(bool); !ok {
		return true
	} else {
		return val
	}
}

// TransmitVendorMetrics returns whether the controller sends charm-collected metrics
// in this model for anonymized aggregate analytics. By default this should be true.
func (c *Config) TransmitVendorMetrics() bool {
	if val, ok := c.defined[TransmitVendorMetricsKey].(bool); !ok {
		return true
	} else {
		return val
	}
}

// ProvisionerHarvestMode reports the harvesting methodology the
// provisioner should take.
func (c *Config) ProvisionerHarvestMode() HarvestMode {
	if v, ok := c.defined[ProvisionerHarvestModeKey].(string); ok {
		if method, err := ParseHarvestMode(v); err != nil {
			// This setting should have already been validated. Don't
			// burden the caller with handling any errors.
			panic(err)
		} else {
			return method
		}
	} else {
		return HarvestDestroyed
	}
}

// ImageStream returns the simplestreams stream
// used to identify which image ids to search
// when starting an instance.
func (c *Config) ImageStream() string {
	v, _ := c.defined["image-stream"].(string)
	if v != "" {
		return v
	}
	return "released"
}

// AgentStream returns the simplestreams stream
// used to identify which tools to use when
// when bootstrapping or upgrading an environment.
func (c *Config) AgentStream() string {
	v, _ := c.defined[AgentStreamKey].(string)
	if v != "" {
		return v
	}
	return "released"
}

// TestMode indicates if the environment is intended for testing.
// In this case, accessing the charm store does not affect statistical
// data of the store.
func (c *Config) TestMode() bool {
	val, _ := c.defined["test-mode"].(bool)
	return val
}

// DisableNetworkManagement reports whether Juju is allowed to
// configure and manage networking inside the environment.
func (c *Config) DisableNetworkManagement() (bool, bool) {
	v, ok := c.defined["disable-network-management"].(bool)
	return v, ok
}

// IgnoreMachineAddresses reports whether Juju will discover
// and store machine addresses on startup.
func (c *Config) IgnoreMachineAddresses() (bool, bool) {
	v, ok := c.defined[IgnoreMachineAddresses].(bool)
	return v, ok
}

// StorageDefaultBlockSource returns the default block storage
// source for the environment.
func (c *Config) StorageDefaultBlockSource() (string, bool) {
	bs := c.asString(StorageDefaultBlockSourceKey)
	return bs, bs != ""
}

// StorageDefaultFilesystemSource returns the default filesystem
// storage source for the environment.
func (c *Config) StorageDefaultFilesystemSource() (string, bool) {
	bs := c.asString(StorageDefaultFilesystemSourceKey)
	return bs, bs != ""
}

// ResourceTags returns a set of tags to set on environment resources
// that Juju creates and manages, if the provider supports them. These
// tags have no special meaning to Juju, but may be used for existing
// chargeback accounting schemes or other identification purposes.
func (c *Config) ResourceTags() (map[string]string, bool) {
	tags, err := c.resourceTags()
	if err != nil {
		panic(err) // should be prevented by Validate
	}
	return tags, tags != nil
}

func (c *Config) resourceTags() (map[string]string, error) {
	v, ok := c.defined[ResourceTagsKey].(map[string]string)
	if !ok {
		return nil, nil
	}
	for k := range v {
		if strings.HasPrefix(k, tags.JujuTagPrefix) {
			return nil, errors.Errorf("tag %q uses reserved prefix %q", k, tags.JujuTagPrefix)
		}
	}
	return v, nil
}

// MaxStatusHistoryAge is the maximum age of status history entries
// before being pruned.
func (c *Config) MaxStatusHistoryAge() time.Duration {
	// Value has already been validated.
	val, _ := time.ParseDuration(c.mustString(MaxStatusHistoryAge))
	return val
}

// MaxStatusHistorySizeMB is the maximum size in MiB which the status history
// collection can grow to before being pruned.
func (c *Config) MaxStatusHistorySizeMB() uint {
	// Value has already been validated.
	val, _ := utils.ParseSize(c.mustString(MaxStatusHistorySize))
	return uint(val)
}

func (c *Config) MaxActionResultsAge() time.Duration {
	// Value has already been validated.
	val, _ := time.ParseDuration(c.mustString(MaxActionResultsAge))
	return val
}

func (c *Config) MaxActionResultsSizeMB() uint {
	// Value has already been validated.
	val, _ := utils.ParseSize(c.mustString(MaxActionResultsSize))
	return uint(val)
}

// UpdateStatusHookInterval is how often to run the charm
// update-status hook.
func (c *Config) UpdateStatusHookInterval() time.Duration {
	// TODO(wallyworld) - remove this work around when possible as
	// we already have a defaulting mechanism for config.
	// It's only here to guard against using Juju clients >= 2.2
	// with Juju controllers running 2.1.x
	raw := c.asString(UpdateStatusHookInterval)
	if raw == "" {
		raw = DefaultUpdateStatusHookInterval
	}
	// Value has already been validated.
	val, _ := time.ParseDuration(raw)
	return val
}

// EgressSubnets are the source addresses from which traffic from this model
// originates if the model is deployed such that NAT or similar is in use.
func (c *Config) EgressSubnets() []string {
	raw := c.asString(EgressSubnets)
	if raw == "" {
		return []string{}
	}
	// Value has already been validated.
	rawAddr := strings.Split(raw, ",")
	result := make([]string, len(rawAddr))
	for i, addr := range rawAddr {
		result[i] = strings.TrimSpace(addr)
	}
	return result
}

// FanConfig is the configuration of FAN network running in the model.
func (c *Config) FanConfig() (network.FanConfig, error) {
	// At this point we are sure that the line is valid.
	return network.ParseFanConfig(c.asString(FanConfig))
}

// UnknownAttrs returns a copy of the raw configuration attributes
// that are supposedly specific to the environment type. They could
// also be wrong attributes, though. Only the specific environment
// implementation can tell.
func (c *Config) UnknownAttrs() map[string]interface{} {
	newAttrs := make(map[string]interface{})
	for k, v := range c.unknown {
		newAttrs[k] = v
	}
	return newAttrs
}

// AllAttrs returns a copy of the raw configuration attributes.
func (c *Config) AllAttrs() map[string]interface{} {
	allAttrs := c.UnknownAttrs()
	for k, v := range c.defined {
		allAttrs[k] = v
	}
	return allAttrs
}

// Remove returns a new configuration that has the attributes of c minus attrs.
func (c *Config) Remove(attrs []string) (*Config, error) {
	defined := c.AllAttrs()
	for _, k := range attrs {
		delete(defined, k)
	}
	return New(NoDefaults, defined)
}

// Apply returns a new configuration that has the attributes of c plus attrs.
func (c *Config) Apply(attrs map[string]interface{}) (*Config, error) {
	defined := c.AllAttrs()
	for k, v := range attrs {
		defined[k] = v
	}
	return New(NoDefaults, defined)
}

// fields holds the validation schema fields derived from configSchema.
var fields = func() schema.Fields {
	combinedSchema, err := Schema(nil)
	if err != nil {
		panic(err)
	}
	fs, _, err := combinedSchema.ValidationSchema()
	if err != nil {
		panic(err)
	}
	return fs
}()

// alwaysOptional holds configuration defaults for attributes that may
// be unspecified even after a configuration has been created with all
// defaults filled out.
//
// This table is not definitive: it specifies those attributes which are
// optional when the config goes through its initial schema coercion,
// but some fields listed as optional here are actually mandatory
// with NoDefaults and are checked at the later Validate stage.
var alwaysOptional = schema.Defaults{
	AgentVersionKey:   schema.Omit,
	AuthorizedKeysKey: schema.Omit,
	ExtraInfoKey:      schema.Omit,

	LogForwardEnabled:      schema.Omit,
	LogFwdSyslogHost:       schema.Omit,
	LogFwdSyslogCACert:     schema.Omit,
	LogFwdSyslogClientCert: schema.Omit,
	LogFwdSyslogClientKey:  schema.Omit,

	// Storage related config.
	// Environ providers will specify their own defaults.
	StorageDefaultBlockSourceKey:      schema.Omit,
	StorageDefaultFilesystemSourceKey: schema.Omit,

	"firewall-mode":              schema.Omit,
	"logging-config":             schema.Omit,
	ProvisionerHarvestModeKey:    schema.Omit,
	HTTPProxyKey:                 schema.Omit,
	HTTPSProxyKey:                schema.Omit,
	FTPProxyKey:                  schema.Omit,
	NoProxyKey:                   schema.Omit,
	AptHTTPProxyKey:              schema.Omit,
	AptHTTPSProxyKey:             schema.Omit,
	AptFTPProxyKey:               schema.Omit,
	AptNoProxyKey:                schema.Omit,
	"apt-mirror":                 schema.Omit,
	AgentStreamKey:               schema.Omit,
	ResourceTagsKey:              schema.Omit,
	"cloudimg-base-url":          schema.Omit,
	"enable-os-refresh-update":   schema.Omit,
	"enable-os-upgrade":          schema.Omit,
	"image-stream":               schema.Omit,
	"image-metadata-url":         schema.Omit,
	AgentMetadataURLKey:          schema.Omit,
	"default-series":             schema.Omit,
	"development":                schema.Omit,
	"ssl-hostname-verification":  schema.Omit,
	"proxy-ssh":                  schema.Omit,
	"disable-network-management": schema.Omit,
	IgnoreMachineAddresses:       schema.Omit,
	AutomaticallyRetryHooks:      schema.Omit,
	"test-mode":                  schema.Omit,
	TransmitVendorMetricsKey:     schema.Omit,
	NetBondReconfigureDelayKey:   schema.Omit,
	ContainerNetworkingMethod:    schema.Omit,
	MaxStatusHistoryAge:          schema.Omit,
	MaxStatusHistorySize:         schema.Omit,
	MaxActionResultsAge:          schema.Omit,
	MaxActionResultsSize:         schema.Omit,
	UpdateStatusHookInterval:     schema.Omit,
	EgressSubnets:                schema.Omit,
	FanConfig:                    schema.Omit,
}

func allowEmpty(attr string) bool {
	return alwaysOptional[attr] == "" || alwaysOptional[attr] == schema.Omit
}

var defaultsWhenParsing = allDefaults()

// allDefaults returns a schema.Defaults that contains
// defaults to be used when creating a new config with
// UseDefaults.
func allDefaults() schema.Defaults {
	d := schema.Defaults{}
	configDefaults := ConfigDefaults()
	for attr, val := range configDefaults {
		d[attr] = val
	}
	for attr, val := range alwaysOptional {
		if _, ok := d[attr]; !ok {
			d[attr] = val
		}
	}
	return d
}

// immutableAttributes holds those attributes
// which are not allowed to change in the lifetime
// of an environment.
var immutableAttributes = []string{
	NameKey,
	TypeKey,
	UUIDKey,
	"firewall-mode",
}

var (
	withDefaultsChecker = schema.FieldMap(fields, defaultsWhenParsing)
	noDefaultsChecker   = schema.FieldMap(fields, alwaysOptional)
)

// ValidateUnknownAttrs checks the unknown attributes of the config against
// the supplied fields and defaults, and returns an error if any fails to
// validate. Unknown fields are warned about, but preserved, on the basis
// that they are reasonably likely to have been written by or for a version
// of juju that does recognise the fields, but that their presence is still
// anomalous to some degree and should be flagged (and that there is thereby
// a mechanism for observing fields that really are typos etc).
func (cfg *Config) ValidateUnknownAttrs(extrafields schema.Fields, defaults schema.Defaults) (map[string]interface{}, error) {
	attrs := cfg.UnknownAttrs()
	checker := schema.FieldMap(extrafields, defaults)
	coerced, err := checker.Coerce(attrs, nil)
	if err != nil {
		// TODO(ericsnow) Drop this?
		logger.Debugf("coercion failed attributes: %#v, checker: %#v, %v", attrs, checker, err)
		return nil, err
	}
	result := coerced.(map[string]interface{})
	for name, value := range attrs {
		if extrafields[name] == nil {
			// We know this name isn't in the global fields, or it wouldn't be
			// an UnknownAttr, it also appears to not be in the extra fields
			// that are provider specific.  Check to see if an alternative
			// spelling is in either the extra fields or the core fields.
			if val, isString := value.(string); isString && val != "" {
				// only warn about attributes with non-empty string values
				altName := strings.Replace(name, "_", "-", -1)
				if extrafields[altName] != nil || fields[altName] != nil {
					logger.Warningf("unknown config field %q, did you mean %q?", name, altName)
				} else {
					logger.Warningf("unknown config field %q", name)
				}
			}
			result[name] = value
		}
	}
	return result, nil
}

// SpecializeCharmRepo customizes a repository for a given configuration.
// It returns a charm repository with test mode enabled if applicable.
func SpecializeCharmRepo(repo charmrepo.Interface, cfg *Config) charmrepo.Interface {
	type specializer interface {
		WithTestMode() charmrepo.Interface
	}
	if store, ok := repo.(specializer); ok {
		if cfg.TestMode() {
			return store.WithTestMode()
		}
	}
	return repo
}

func addIfNotEmpty(settings map[string]interface{}, key, value string) {
	if value != "" {
		settings[key] = value
	}
}

// ProxyConfigMap returns a map suitable to be applied to a Config to update
// proxy settings.
func ProxyConfigMap(proxySettings proxy.Settings) map[string]interface{} {
	settings := make(map[string]interface{})
	addIfNotEmpty(settings, HTTPProxyKey, proxySettings.Http)
	addIfNotEmpty(settings, HTTPSProxyKey, proxySettings.Https)
	addIfNotEmpty(settings, FTPProxyKey, proxySettings.Ftp)
	addIfNotEmpty(settings, NoProxyKey, proxySettings.NoProxy)
	return settings
}

// AptProxyConfigMap returns a map suitable to be applied to a Config to update
// proxy settings.
func AptProxyConfigMap(proxySettings proxy.Settings) map[string]interface{} {
	settings := make(map[string]interface{})
	addIfNotEmpty(settings, AptHTTPProxyKey, proxySettings.Http)
	addIfNotEmpty(settings, AptHTTPSProxyKey, proxySettings.Https)
	addIfNotEmpty(settings, AptFTPProxyKey, proxySettings.Ftp)
	addIfNotEmpty(settings, AptNoProxyKey, proxySettings.NoProxy)
	return settings
}

// Schema returns a configuration schema that includes both
// the given extra fields and all the fields defined in this package.
// It returns an error if extra defines any fields defined in this
// package.
func Schema(extra environschema.Fields) (environschema.Fields, error) {
	fields := make(environschema.Fields)
	for name, field := range configSchema {
		if controller.ControllerOnlyAttribute(name) {
			return nil, errors.Errorf("config field %q clashes with controller config", name)
		}
		fields[name] = field
	}
	for name, field := range extra {
		if controller.ControllerOnlyAttribute(name) {
			return nil, errors.Errorf("config field %q clashes with controller config", name)
		}
		if _, ok := fields[name]; ok {
			return nil, errors.Errorf("config field %q clashes with global config", name)
		}
		fields[name] = field
	}
	return fields, nil
}

// configSchema holds information on all the fields defined by
// the config package.
// TODO(rog) make this available to external packages.
var configSchema = environschema.Fields{
	AgentMetadataURLKey: {
		Description: "URL of private stream",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	AgentStreamKey: {
		Description: `Version of Juju to use for deploy/upgrades.`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	AgentVersionKey: {
		Description: "The desired Juju agent version to use",
		Type:        environschema.Tstring,
		Group:       environschema.JujuGroup,
		Immutable:   true,
	},
	AptFTPProxyKey: {
		// TODO document acceptable format
		Description: "The APT FTP proxy for the model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	AptHTTPProxyKey: {
		// TODO document acceptable format
		Description: "The APT HTTP proxy for the model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	AptHTTPSProxyKey: {
		// TODO document acceptable format
		Description: "The APT HTTPS proxy for the model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	AptNoProxyKey: {
		Description: "List of domain addresses not to be proxied for APT (comma-separated)",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"apt-mirror": {
		// TODO document acceptable format
		Description: "The APT mirror for the model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	AuthorizedKeysKey: {
		Description: "Any authorized SSH public keys for the model, as found in a ~/.ssh/authorized_keys file",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"default-series": {
		Description: "The default series of Ubuntu to use for deploying charms",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"development": {
		Description: "Whether the model is in development mode",
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	"disable-network-management": {
		Description: "Whether the provider should control networks (on MAAS models, set to true for MAAS to control networks",
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	IgnoreMachineAddresses: {
		Description: "Whether the machine worker should discover machine addresses on startup",
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	"enable-os-refresh-update": {
		Description: `Whether newly provisioned instances should run their respective OS's update capability.`,
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	"enable-os-upgrade": {
		Description: `Whether newly provisioned instances should run their respective OS's upgrade capability.`,
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	ExtraInfoKey: {
		Description: "Arbitrary user specified string data that is stored against the model.",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"firewall-mode": {
		Description: `The mode to use for network firewalling.

'instance' requests the use of an individual firewall per instance.

'global' uses a single firewall for all instances (access
for a network port is enabled to one instance if any instance requires
that port).

'none' requests that no firewalling should be performed
inside the model. It's useful for clouds without support for either
global or per instance security groups.`,
		Type:      environschema.Tstring,
		Values:    []interface{}{FwInstance, FwGlobal, FwNone},
		Immutable: true,
		Group:     environschema.EnvironGroup,
	},
	FTPProxyKey: {
		Description: "The FTP proxy value to configure on instances, in the FTP_PROXY environment variable",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	HTTPProxyKey: {
		Description: "The HTTP proxy value to configure on instances, in the HTTP_PROXY environment variable",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	HTTPSProxyKey: {
		Description: "The HTTPS proxy value to configure on instances, in the HTTPS_PROXY environment variable",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"image-metadata-url": {
		Description: "The URL at which the metadata used to locate OS image ids is located",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"image-stream": {
		Description: `The simplestreams stream used to identify which image ids to search when starting an instance.`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"logging-config": {
		Description: `The configuration string to use when configuring Juju agent logging (see http://godoc.org/github.com/juju/loggo#ParseConfigurationString for details)`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	NameKey: {
		Description: "The name of the current model",
		Type:        environschema.Tstring,
		Mandatory:   true,
		Immutable:   true,
		Group:       environschema.EnvironGroup,
	},
	NoProxyKey: {
		Description: "List of domain addresses not to be proxied (comma-separated)",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	ProvisionerHarvestModeKey: {
		// default: destroyed, but also depends on current setting of ProvisionerSafeModeKey
		Description: "What to do with unknown machines. See https://jujucharms.com/docs/stable/config-general#juju-lifecycle-and-harvesting (default destroyed)",
		Type:        environschema.Tstring,
		Values:      []interface{}{"all", "none", "unknown", "destroyed"},
		Group:       environschema.EnvironGroup,
	},
	"proxy-ssh": {
		// default: true
		Description: `Whether SSH commands should be proxied through the API server`,
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	ResourceTagsKey: {
		Description: "resource tags",
		Type:        environschema.Tattrs,
		Group:       environschema.EnvironGroup,
	},
	LogForwardEnabled: {
		Description: `Whether syslog forwarding is enabled.`,
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	LogFwdSyslogHost: {
		Description: `The hostname:port of the syslog server.`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	LogFwdSyslogCACert: {
		Description: `The certificate of the CA that signed the syslog server certificate, in PEM format.`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	LogFwdSyslogClientCert: {
		Description: `The syslog client certificate in PEM format.`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	LogFwdSyslogClientKey: {
		Description: `The syslog client key in PEM format.`,
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"ssl-hostname-verification": {
		Description: "Whether SSL hostname verification is enabled (default true)",
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	StorageDefaultBlockSourceKey: {
		Description: "The default block storage source for the model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	StorageDefaultFilesystemSourceKey: {
		Description: "The default filesystem storage source for the model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	"test-mode": {
		Description: `Whether the model is intended for testing.
If true, accessing the charm store does not affect statistical
data of the store. (default false)`,
		Type:  environschema.Tbool,
		Group: environschema.EnvironGroup,
	},
	TypeKey: {
		Description: "Type of model, e.g. local, ec2",
		Type:        environschema.Tstring,
		Mandatory:   true,
		Immutable:   true,
		Group:       environschema.EnvironGroup,
	},
	UUIDKey: {
		Description: "The UUID of the model",
		Type:        environschema.Tstring,
		Group:       environschema.JujuGroup,
		Immutable:   true,
	},
	AutomaticallyRetryHooks: {
		Description: "Determines whether the uniter should automatically retry failed hooks",
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	TransmitVendorMetricsKey: {
		Description: "Determines whether metrics declared by charms deployed into this model are sent for anonymized aggregate analytics",
		Type:        environschema.Tbool,
		Group:       environschema.EnvironGroup,
	},
	NetBondReconfigureDelayKey: {
		Description: "The amount of time in seconds to sleep between ifdown and ifup when bridging",
		Type:        environschema.Tint,
		Group:       environschema.EnvironGroup,
	},
	ContainerNetworkingMethod: {
		Description: "Method of container networking setup - one of fan, provider, local",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	MaxStatusHistoryAge: {
		Description: "The maximum age for status history entries before they are pruned, in human-readable time format",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	MaxStatusHistorySize: {
		Description: "The maximum size for the status history collection, in human-readable memory format",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	MaxActionResultsAge: {
		Description: "The maximum age for action entries before they are pruned, in human-readable time format",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	MaxActionResultsSize: {
		Description: "The maximum size for the action collection, in human-readable memory format",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	UpdateStatusHookInterval: {
		Description: "How often to run the charm update-status hook, in human-readable time format (default 5m, range 1-60m)",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	EgressSubnets: {
		Description: "Source address(es) for traffic originating from this model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
	FanConfig: {
		Description: "Configuration for fan networking for this model",
		Type:        environschema.Tstring,
		Group:       environschema.EnvironGroup,
	},
}
