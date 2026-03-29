package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Config struct {
	System System  `yaml:"system"`
	Relays []Relay `yaml:"relays"`
}

type System struct {
	IPForward    bool         `yaml:"ip_forward"`
	DefaultRoute DefaultRoute `yaml:"default_route"`
}

type DefaultRoute struct {
	Tun    string `yaml:"tun"`
	Except string `yaml:"except"`
}

type Relay struct {
	Ingress IngressEndpoint `yaml:"ingress"`
	Egress  EgressEndpoint  `yaml:"egress"`
}

type TunEndpoint struct {
	Type string `yaml:"type"`
	Name string `yaml:"name"`
	CIDR string `yaml:"cidr"`
	Peer string `yaml:"peer"`
}

type TunIngress struct {
	TunEndpoint `yaml:",inline"`
}

type TunEgress struct {
	TunEndpoint `yaml:",inline"`
	NAT         NAT `yaml:"nat"`
}

type NAT struct {
	Forward  NATActions `yaml:"forward"`
	Backward NATActions `yaml:"backward"`
}

type NATActions struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

type UDPEndpoint struct {
	Type     string `yaml:"type"`
	Password string `yaml:"password"`
}

type UDPIngress struct {
	UDPEndpoint `yaml:",inline"`
	Listen      string `yaml:"listen"`
}

type UDPEgress struct {
	UDPEndpoint `yaml:",inline"`
	Dial        string `yaml:"dial"`
}

type NullEndpoint struct {
}

type EndpointValue struct {
	Value any
}

type IngressEndpoint struct {
	EndpointValue
}

type EgressEndpoint struct {
	EndpointValue
}

func (ew *IngressEndpoint) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Type string `yaml:"type"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	switch raw.Type {
	case "tun":
		var ep TunIngress
		if err := value.Decode(&ep); err != nil {
			return err
		}
		ew.Value = ep
	case "udp":
		var ep UDPIngress
		if err := value.Decode(&ep); err != nil {
			return err
		}
		ew.Value = ep
	case "null":
		var ep NullEndpoint
		if err := value.Decode(&ep); err != nil {
			return err
		}
		ew.Value = ep
	default:
		return fmt.Errorf("unknown endpoint type: %s", raw.Type)
	}
	return nil
}

func (ew *EgressEndpoint) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Type string `yaml:"type"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	switch raw.Type {
	case "tun":
		var ep TunEgress
		if err := value.Decode(&ep); err != nil {
			return err
		}
		ew.Value = ep
	case "udp":
		var ep UDPEgress
		if err := value.Decode(&ep); err != nil {
			return err
		}
		ew.Value = ep
	case "null":
		var ep NullEndpoint
		if err := value.Decode(&ep); err != nil {
			return err
		}
		ew.Value = ep
	default:
		return fmt.Errorf("unknown endpoint type: %s", raw.Type)
	}
	return nil
}

func UnmarshalConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	return &cfg, nil
}
