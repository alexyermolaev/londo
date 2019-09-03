package config

type (
	mongoConf struct {
		Hostname string
		Username string
		Password string
		Port     int
	}

	amqpConf struct {
		Hostname string
		Username string
		Password string
		Exchange string
		Port     int
	}

	endpointsConf struct {
		Enroll  string
		Revoke  string
		Collect string
	}

	caApiConf struct {
		Url         string
		Username    string
		Password    string
		CustomerUri string
		Endpoints   endpointsConf
	}

	certParamsConf struct {
		Country            string
		Province           string
		Locality           string
		StreetAddress      string
		Organization       string
		OrganizationalUnit string
		FormatType         string
		Comments           string
		SingleDomainType   int
		MultiDomainType    int
		PostalCode         int
		OrgId              int
		Term               int
		BitSize            int
	}

	grpcConf struct {
		Port int
	}

	jwtConf struct {
		Sub, Aud string
		Exp      int
	}

	config struct {
		MongoDB    mongoConf      `yaml:"mongodb"`
		AMQP       amqpConf       `yaml:"amqp"`
		CAAPI      caApiConf      `yaml:"sectigo"`
		CertParams certParamsConf `yaml:"cert_params"`
		GRPC       grpcConf       `yaml:"grpc"`
	}
)
