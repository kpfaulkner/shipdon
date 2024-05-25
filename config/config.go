package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	InstanceURL  string `json:"instanceURL"`
	ClientID     string `json:"appID"`
	ClientSecret string `json:"appSecret"`
	Token        string `json:"token"`

	// DarkMode or LightMode... only 2 options for now.
	DarkMode bool `json:"darkMode"`

	// We get the list of lists from the server, but we don't want to display all of them.
	// Should probably be a map, but leave as slice for now
	ListsToNotDisplay []string `json:"listsToNotDisplay"`

	// username and password for the user
	// Given this is NOT secure, I'd really not recommend using this for now. OAuth2 is the way to go.
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoadConfig() *Config {
	data, err := os.ReadFile("config.json")
	if err != nil {

		// make a dummy...
		c := Config{
			InstanceURL:       "",
			ClientID:          "",
			ClientSecret:      "",
			Token:             "",
			DarkMode:          false,
			ListsToNotDisplay: []string{},
			Username:          "",
			Password:          "",
		}
		c.Save()
		return &c
	}

	var c Config
	err = json.Unmarshal(data, &c)
	if err != nil {
		panic("unable to unmarshal config file")
	}
	return &c
}

func (c *Config) Save() error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	err = os.WriteFile("config.json", data, 0644)
	if err != nil {
		return err
	}
	return nil
}
