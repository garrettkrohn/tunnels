package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Databases map[string]DatabaseConfig `yaml:"databases"`
}

type DatabaseConfig struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	User             string `yaml:"user"`
	SSHJumpHost      string `yaml:"ssh_jump_host"`
	SSHJumpPort      int    `yaml:"ssh_jump_port"`
	LocalPort        int    `yaml:"local_port"`
	PasswordPassPath string `yaml:"password_pass_path"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s [-p] <db_name>", os.Args[0])
	}

	var dbName string
	var insertToPass bool
	var passName string

	// Check if first argument is "-p" for clipboard-to-pass functionality
	if os.Args[1] == "-p" {
		if len(os.Args) < 4 {
			log.Fatalf("Usage: %s -p <password> <db_name>", os.Args[0])
		}
		insertToPass = true
		dbName = os.Args[3]
		passName = os.Args[2]
	} else {
		dbName = os.Args[1]
	}

	// If -p flag is used, insert clipboard to pass and exit
	if insertToPass {
		passCmd := exec.Command("pass", "insert", "-m", "-f", passName)
		// passCmd.Stdin = strings.NewReader(string(clipboardContent))

		if err := passCmd.Run(); err != nil {
			fmt.Errorf("failed to insert into pass: %v", err)
		}
	}

	usr, err := user.Current()
	if err != nil {
		log.Fatalf("Error getting current user: %v", err)
	}

	configPath := filepath.Join(usr.HomeDir, "code", "tunnels", "config.yaml")
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	dbConfig, ok := cfg.Databases[dbName]
	if !ok {
		log.Fatalf("Database config for %q not found", dbName)
	}

	// Get password from pass
	dbPassword, err := getPassPassword(dbConfig.PasswordPassPath)
	if err != nil {
		log.Fatalf("Error getting password from pass: %v", err)
	}

	fmt.Printf("Starting SSH tunnel to %s...\n", dbName)

	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=60",
		fmt.Sprintf("-L%d:%s:%d", dbConfig.LocalPort, dbConfig.Host, dbConfig.Port),
		fmt.Sprintf("%s@%s", dbConfig.User, dbConfig.SSHJumpHost),
	}

	// Print full SSH command
	fmt.Printf("Full SSH command: ssh %s\n", strings.Join(sshArgs, " "))

	// Execute SSH tunnel (foreground)
	tunnelCmd := exec.Command("ssh", sshArgs...)
	tunnelCmd.Stdout = os.Stdout
	tunnelCmd.Stderr = os.Stderr
	tunnelCmd.Stdin = os.Stdin // needed if you want to type passwords for jump host

	if err := tunnelCmd.Run(); err != nil {
		log.Fatalf("Error starting SSH tunnel: %v", err)
	}

	fmt.Printf("Tunnel established: localhost:%d -> %s:%d\n",
		dbConfig.LocalPort, dbConfig.Host, dbConfig.Port)

	// Launch psql
	fmt.Println("Launching psql...")
	psqlCmd := exec.Command(
		"psql",
		"-h", "localhost",
		"-p", fmt.Sprintf("%d", dbConfig.LocalPort),
		"-U", dbConfig.User,
	)
	psqlCmd.Stdout = os.Stdout
	psqlCmd.Stderr = os.Stderr
	psqlCmd.Stdin = os.Stdin

	// Pass the password via environment variable
	psqlCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbPassword))

	if err := psqlCmd.Run(); err != nil {
		log.Fatalf("Error launching psql: %v", err)
	}

}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func getPassPassword(path string) (string, error) {
	cmd := exec.Command("pass", "show", path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
