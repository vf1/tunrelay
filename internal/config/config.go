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
	Masquerade   Masquerade   `yaml:"masquerade"`
}

type DefaultRoute struct {
	Tun    string `yaml:"tun"`
	Except string `yaml:"except"`
}

type Masquerade struct {
	SAddr   string `yaml:"saddr"`
	OIFName string `yaml:"oifname"`
}

type Relay struct {
	Ingress     IngressEndpoint `yaml:"ingress"`
	Middlewares []Middleware    `yaml:"middlewares"`
	Egress      EgressEndpoint  `yaml:"egress"`
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
}

type UDPIngress struct {
	Type   string `yaml:"type"`
	Listen string `yaml:"listen"`
	Peers  []Peer `yaml:"peers"`
}

type Peer struct {
	SAddr    string `yaml:"saddr"`
	Password string `yaml:"password"`
}

type UDPEgress struct {
	Type     string `yaml:"type"`
	Password string `yaml:"password"`
	Dial     string `yaml:"dial"`
	AllowSrc string `yaml:"allow_src"`
}

type ReplaceIP struct {
	ForwardSrc  string `yaml:"forward_src"`
	ForwardDst  string `yaml:"forward_dst"`
	BackwardSrc string `yaml:"backward_src"`
	BackwardDst string `yaml:"backward_dst"`
}

type NAT struct {
	SrcRangeStart       string `yaml:"src_range_start"`
	SrcRangeEnd         string `yaml:"src_range_end"`
	SrcRangeStartDarwin string `yaml:"src_range_start_darwin"`
	SrcRangeEndDarwin   string `yaml:"src_range_end_darwin"`
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

type Middleware struct {
	Value any
}

type rawType struct {
	Type string `yaml:"type"`
}

func (ew *IngressEndpoint) UnmarshalYAML(value *yaml.Node) error {
	var raw rawType
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
	var raw rawType
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

func (mv *Middleware) UnmarshalYAML(value *yaml.Node) error {
	var raw rawType
	if err := value.Decode(&raw); err != nil {
		return err
	}

	switch raw.Type {
	case "replace_ip":
		var m ReplaceIP
		if err := value.Decode(&m); err != nil {
			return err
		}
		mv.Value = m
	case "nat":
		var m NAT
		if err := value.Decode(&m); err != nil {
			return err
		}
		mv.Value = m
	default:
		return fmt.Errorf("unknown middleware type: %s", raw.Type)
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
