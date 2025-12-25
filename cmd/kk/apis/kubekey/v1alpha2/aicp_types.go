package v1alpha2

import (
	"net"
)

type CertKeys string

const (
	GeneralCertKey CertKeys = "General"
	AiCertKey      CertKeys = DefaultAicpDomainAi
	ApiCertKey     CertKeys = DefaultAicpDomainApi
	ConsoleCertKey CertKeys = DefaultAicpDomainConsole
	BossCertKey    CertKeys = DefaultAicpDomainBoss
	CadminCertKey  CertKeys = DefaultAicpDomainCadmin
	DocsCertKey    CertKeys = DefaultAicpDomainDocs
)

type CertFile struct {
	KeyFile  string `yaml:"keyFile" json:"keyFile,omitempty"`
	CertFile string `yaml:"certFile" json:"certFile,omitempty"`
}

type DomainConfig struct {
	Domain    string                `yaml:"domain" json:"domain,omitempty"`
	Ai        string                `yaml:"ai" json:"ai,omitempty"`
	Api       string                `yaml:"api" json:"api,omitempty"`
	Console   string                `yaml:"console" json:"console,omitempty"`
	Boss      string                `yaml:"boss" json:"boss,omitempty"`
	Cadmin    string                `yaml:"cadmin" json:"cadmin,omitempty"`
	Docs      string                `yaml:"docs" json:"docs,omitempty"`
	Protocol  string                `yaml:"protocol" json:"protocol,omitempty"`
	CertPaths map[CertKeys]CertFile `yaml:"certPaths" json:"certPaths,omitempty"`
}

type Aicp struct {
	Zone         string       `yaml:"zone" json:"zone,omitempty"`
	Billing      bool         `yaml:"billing" json:"billing,omitempty"`
	Hami         bool         `yaml:"hami" json:"hami,omitempty"`
	Network      bool         `yaml:"network" json:"network,omitempty"`
	DomainConfig DomainConfig `yaml:"domainConfig" json:"domainConfig,omitempty"`
	HostIp       *net.IP      `yaml:"hostIp,omitempty" json:"hostIp,omitempty"`
}
