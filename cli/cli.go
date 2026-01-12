package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var rootCmd = &cobra.Command{
	Use:   "poxy",
	Short: "Poxy is a universal package manager for linux distrobutions",
	Long:  `Poxy discover's linux distro's packaging system and unifies each command based on the systems package manager`,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(detectSystemCommand)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
}
func Execute() error {
	return rootCmd.Execute()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints current version number for Poxy",
	Run: func(cmd *cobra.Command, args []string) {
		version := "0.1.0"
		versionStr := fmt.Sprintf("Version:  %s", version)
		fmt.Println(versionStr)
	},
}

var detectSystemCommand = &cobra.Command{
	Use:   "system",
	Short: "prints the current system that poxy detects",
	Long:  `Uses system os functionatity to detect the current system poxy detects`,
	Run: func(cmd *cobra.Command, args []string) {
		currentOS := runtime.GOOS
		switch currentOS {
		case "windows":
			osAndRuntime := fmt.Sprintf("Current OS: %s Architecture: %s", currentOS, runtime.GOARCH)
			log.Println(osAndRuntime)
		case "linux":
			linuxDistro := currentDistro()
			osAndRuntime := fmt.Sprintf("Current OS: %s, Architecture: %s, Linux Distrobution: %s", strings.ToUpper(currentOS), strings.ToUpper(runtime.GOARCH), strings.ToUpper(linuxDistro))
			log.Println(osAndRuntime)
		case "darwin":
			log.Println("Mac OS")
		default:
			log.Println("Unknown operating system")
		}
	},
}

func currentDistro() string {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		log.Fatal(err)
	}
	string_map := make(map[string]string)
	s := string(raw)
	for line := range strings.Lines(s) {
		pair := strings.Split(line, "=")
		k := pair[0]
		v := pair[1]
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		string_map[k] = v
	}
	return string_map["ID"]
}

func DetectPackageManager() []string {
	distro := currentDistro()
	var systemPackageManager []string
	switch distro {
	case "arch", "archlinux":
		systemPackageManager = []string{"pacman"}
	case "debian", "ubuntu":
		systemPackageManager = []string{"apt", "apt-get"}
	case "fedora", "redhat":
		systemPackageManager = []string{"dnf", "yum"}
	default:
		systemPackageManager = []string{}
	}
	return systemPackageManager
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Installs commands based on system package manager",
	Long:  `Uses system package manager to install a package of the name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		packageManagers := DetectPackageManager()
		var autoinstall string
		if len(packageManagers) == 0 {
			log.Fatal("No system package manager was detected")
		}
		var installArg string
		if len(packageManagers) >= 1 {
			preferedPac := packageManagers[0]
			switch preferedPac {
			case "apt", "apt-get", "dnf", "yum":
				installArg = "install"
				autoinstall = "-y"
			case "pacman", "yay":
				installArg = "-S"
				autoinstall = "--noconfirm"
			default:
				return fmt.Errorf("Unsupported package manager: %s", preferedPac)

			}

			for _, pac := range args {
				log.Printf("Installing package: %s \n", pac)
				cmdExec := exec.Command(preferedPac, installArg, autoinstall, pac)
				cmdExec.Stdout = os.Stdout
				cmdExec.Stderr = os.Stderr
				if err := cmdExec.Run(); err != nil {
					return err
				}
			}
		}
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates the system's packages",
	Long:  `Updates the system's packages and allow for the registries to be refreshed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		packageManagers := DetectPackageManager()
		if len(packageManagers) == 0 {
			log.Fatal("No system package manager was detected.")
		}
		var updateCmd string
		if len(packageManagers) >= 1 {
			preferedPac := packageManagers[0]
			switch preferedPac {
			case "apt", "apt-get":
				updateCmd = "update"
			case "dnf", "yum":
				updateCmd = "upgrade"
			case "pacman", "yay":
				updateCmd = "-Syu"
			default:
				return fmt.Errorf("Unable to find update command due to Unsupported package manager: %s", preferedPac)
			}
			cmdExec := exec.Command(preferedPac, updateCmd)
			cmdExec.Stdout = os.Stdout
			cmdExec.Stderr = os.Stderr
			if err := cmdExec.Run(); err != nil {
				return err
			}
		}
		return nil
	},
}
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the system's package",
	Long:  `Uninstall the system's package in a a uninfied uninstall command`,
	RunE: func(cmd *cobra.Command, args []string) error {
		packageManagers := DetectPackageManager()
		if len(packageManagers) == 0 {
			log.Fatal("No system package manager was detected.")
		}
		var removeCmd string
		if len(packageManagers) >= 1 {
			preferedPac := packageManagers[0]
			switch preferedPac {
			case "apt", "apt-get":
				removeCmd = "remove"
			case "dnf", "yum":
				removeCmd = "remove"
			case "pacman", "yay":
				removeCmd = "-R"
			default:
				return fmt.Errorf("Unable to remove command due to Unsupported package manager: %s", preferedPac)
			}
			for _, pac := range args {
				log.Printf("Installing package: %s \n", pac)
				cmdExec := exec.Command(preferedPac, removeCmd, pac)
				cmdExec.Stdout = os.Stdout
				cmdExec.Stderr = os.Stderr
				if err := cmdExec.Run(); err != nil {
					return err
				}
			}
		}
		return nil
	},
}
