package clickhouse

import (
	"fmt"
	"github.com/yadunandan004/scaffold/logger"
	"os"
	"os/exec"
)

type MigrationRunner struct {
	config *Config
}

func NewMigrationRunner(cfg *Config) *MigrationRunner {
	return &MigrationRunner{
		config: cfg,
	}
}

func (m *MigrationRunner) Run() error {
	logger.LogInfo(nil, "Running ClickHouse migrations with Flyway...")

	if !m.isFlywayAvailable() {
		logger.LogInfo(nil, "Flyway not found. Skipping ClickHouse migrations.")
		logger.LogInfo(nil, "To run migrations, install Flyway: https://flywaydb.org/documentation/usage/commandline/")
		return nil
	}

	cmd := m.buildFlywayCommand("migrate")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("flyway migration failed: %w\nOutput: %s", err, string(output))
	}

	logger.LogInfo(nil, "ClickHouse Flyway migration output:\n%s", string(output))
	logger.LogInfo(nil, "ClickHouse migrations completed successfully")

	return nil
}

func (m *MigrationRunner) Info() error {
	if !m.isFlywayAvailable() {
		return fmt.Errorf("flyway not available")
	}

	cmd := m.buildFlywayCommand("info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("flyway info failed: %w\nOutput: %s", err, string(output))
	}

	logger.LogInfo(nil, "ClickHouse Flyway info:\n%s", string(output))
	return nil
}

func (m *MigrationRunner) Validate() error {
	if !m.isFlywayAvailable() {
		return fmt.Errorf("flyway not available")
	}

	cmd := m.buildFlywayCommand("validate")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("flyway validate failed: %w\nOutput: %s", err, string(output))
	}

	logger.LogInfo(nil, "ClickHouse Flyway validation passed")
	return nil
}

func (m *MigrationRunner) Clean() error {
	if !m.isFlywayAvailable() {
		return fmt.Errorf("flyway not available")
	}

	logger.LogInfo(nil, "WARNING: Cleaning ClickHouse database schema...")

	cmd := m.buildFlywayCommand("clean")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("flyway clean failed: %w\nOutput: %s", err, string(output))
	}

	logger.LogInfo(nil, "ClickHouse database schema cleaned")
	return nil
}

func (m *MigrationRunner) isFlywayAvailable() bool {
	_, err := exec.LookPath("flyway")
	return err == nil
}

func (m *MigrationRunner) buildFlywayCommand(command string) *exec.Cmd {
	args := []string{
		command,
		fmt.Sprintf("-url=jdbc:clickhouse://%s:%d/%s", m.config.Host, m.config.Port, m.config.Database),
		fmt.Sprintf("-user=%s", m.config.Username),
		fmt.Sprintf("-password=%s", m.config.Password),
		"-locations=filesystem:./migration/clickhouse",
		"-baselineOnMigrate=true",
		"-table=flyway_schema_history",
	}

	cmd := exec.Command("flyway", args...)
	cmd.Env = os.Environ()

	return cmd
}
