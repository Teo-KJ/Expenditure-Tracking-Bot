package config

// DatabaseConfig define the configs needed to connect to the DB
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"ssl_mode"`
}

type FeaturesConfig struct {
	/*EnableCache bool `yaml:"enableCache"`
	MaxItems    int  `yaml:"maxItems"`*/
	SaveToDB bool `yaml:"save_to_database"`
}

/*type ServerConfig struct {
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"logLevel"`
}
*/
