// Package commands builds MDM command plists (single place — DRY).
package commands

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/micromdm/plist"
)

// InstallProfile builds an InstallProfile command for a .mobileconfig payload.
func InstallProfile(profile []byte) ([]byte, error) {
	if len(profile) == 0 {
		return nil, fmt.Errorf("profile payload required")
	}
	cmd := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command": map[string]any{
			"RequestType": "InstallProfile",
			"Payload":     profile,
		},
	}
	return plist.Marshal(cmd)
}

// RemoveProfile builds a RemoveProfile command.
func RemoveProfile(identifier string) ([]byte, error) {
	if identifier == "" {
		return nil, fmt.Errorf("identifier is required")
	}
	cmd := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command": map[string]any{
			"RequestType": "RemoveProfile",
			"Identifier":  identifier,
		},
	}
	return plist.Marshal(cmd)
}

// Simple builds a command with only RequestType (and CommandUUID).
func Simple(requestType string) ([]byte, error) {
	if requestType == "" {
		return nil, fmt.Errorf("request type required")
	}
	cmd := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command": map[string]any{
			"RequestType": requestType,
		},
	}
	return plist.Marshal(cmd)
}

// DeviceInformation requests common device info queries.
// Returns the command plist and its CommandUUID for result polling.
func DeviceInformation() (cmd []byte, commandUUID string, err error) {
	commandUUID = uuid.NewString()
	payload := map[string]any{
		"CommandUUID": commandUUID,
		"Command": map[string]any{
			"RequestType": "DeviceInformation",
			"Queries": []string{
				"UDID", "SerialNumber", "DeviceName", "OSVersion", "BuildVersion",
				"Model", "ModelName", "ProductName", "IsSupervised",
				"BatteryLevel", "DeviceCapacity", "AvailableDeviceCapacity",
				"WiFiMAC", "BluetoothMAC", "EthernetMACs",
				"IMEI", "MEID", "IsActivationLockEnabled", "IsCloudBackupEnabled",
				"IsDeviceLocatorServiceEnabled", "IsDoNotDisturbInEffect",
				"IsNetworkTethered", "TimeZone", "LocalHostName",
				"SupplementalBuildVersion", "SupplementalOSVersionExtra",
			},
		},
	}
	cmd, err = plist.Marshal(payload)
	return cmd, commandUUID, err
}

// ProfileList lists installed profiles.
func ProfileList() ([]byte, error) {
	return Simple("ProfileList")
}

// InstalledApplicationList lists installed apps.
func InstalledApplicationList() ([]byte, error) {
	return Simple("InstalledApplicationList")
}

// DeviceLock builds a DeviceLock command. PIN is optional (6 digits when set).
func DeviceLock(pin, message string) ([]byte, error) {
	cmdBody := map[string]any{
		"RequestType": "DeviceLock",
	}
	if pin = strings.TrimSpace(pin); pin != "" {
		cmdBody["PIN"] = pin
	}
	if message = strings.TrimSpace(message); message != "" {
		cmdBody["Message"] = message
	}
	payload := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command":     cmdBody,
	}
	return plist.Marshal(payload)
}

// ClearPasscode clears the device passcode (supervised).
func ClearPasscode() ([]byte, error) {
	return Simple("ClearPasscode")
}

// RestartDevice reboots the device (supervised).
func RestartDevice() ([]byte, error) {
	return Simple("RestartDevice")
}

// ShutDownDevice powers off the device (supervised).
func ShutDownDevice() ([]byte, error) {
	return Simple("ShutDownDevice")
}

// EraseDevice wipes the device. PIN is optional (6 digits).
func EraseDevice(pin string) ([]byte, error) {
	cmdBody := map[string]any{
		"RequestType": "EraseDevice",
	}
	if pin = strings.TrimSpace(pin); pin != "" {
		cmdBody["PIN"] = pin
	}
	payload := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command":     cmdBody,
	}
	return plist.Marshal(payload)
}

// EnableLostMode turns on lost mode. Message is required by Apple on supervised devices.
func EnableLostMode(message, phone, footnote string) ([]byte, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	cmdBody := map[string]any{
		"RequestType": "EnableLostMode",
		"Message":     message,
	}
	if phone = strings.TrimSpace(phone); phone != "" {
		cmdBody["PhoneNumber"] = phone
	}
	if footnote = strings.TrimSpace(footnote); footnote != "" {
		cmdBody["Footnote"] = footnote
	}
	payload := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command":     cmdBody,
	}
	return plist.Marshal(payload)
}

// DisableLostMode turns off lost mode.
func DisableLostMode() ([]byte, error) {
	return Simple("DisableLostMode")
}

// PlayLostModeSound plays the lost-mode sound.
func PlayLostModeSound() ([]byte, error) {
	return Simple("PlayLostModeSound")
}

// DeviceLocation requests location while in lost mode.
// Returns the command plist and its CommandUUID for result polling.
func DeviceLocation() (cmd []byte, commandUUID string, err error) {
	commandUUID = uuid.NewString()
	payload := map[string]any{
		"CommandUUID": commandUUID,
		"Command": map[string]any{
			"RequestType": "DeviceLocation",
		},
	}
	cmd, err = plist.Marshal(payload)
	return cmd, commandUUID, err
}

// SecurityInfo requests security-related device info.
func SecurityInfo() (cmd []byte, commandUUID string, err error) {
	commandUUID = uuid.NewString()
	payload := map[string]any{
		"CommandUUID": commandUUID,
		"Command": map[string]any{
			"RequestType": "SecurityInfo",
		},
	}
	cmd, err = plist.Marshal(payload)
	return cmd, commandUUID, err
}

// ProfileListWithUUID is ProfileList returning the CommandUUID for polling.
func ProfileListWithUUID() (cmd []byte, commandUUID string, err error) {
	return simpleWithUUID("ProfileList")
}

// InstalledApplicationListWithUUID lists apps and returns CommandUUID for polling.
func InstalledApplicationListWithUUID() (cmd []byte, commandUUID string, err error) {
	return simpleWithUUID("InstalledApplicationList")
}

func simpleWithUUID(requestType string) (cmd []byte, commandUUID string, err error) {
	if requestType == "" {
		return nil, "", fmt.Errorf("request type required")
	}
	commandUUID = uuid.NewString()
	payload := map[string]any{
		"CommandUUID": commandUUID,
		"Command": map[string]any{
			"RequestType": requestType,
		},
	}
	cmd, err = plist.Marshal(payload)
	return cmd, commandUUID, err
}

func managedAppConfiguration(configurations map[string]string) map[string]any {
	if len(configurations) == 0 {
		return nil
	}
	cfg := map[string]any{}
	for k, v := range configurations {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		cfg[k] = v
	}
	if len(cfg) == 0 {
		return nil
	}
	return cfg
}

// InstallApplication installs an App Store / Custom App by iTunes Adam ID.
// configurations is Managed App Config (com.apple.configuration.managed keys).
// purchaseMethod 1 = VPP/Apps & Books assignment (required for org apps).
func InstallApplication(iTunesStoreID int64, configurations map[string]string, purchaseMethod int) ([]byte, error) {
	if iTunesStoreID <= 0 {
		return nil, fmt.Errorf("iTunesStoreID required")
	}
	cmdBody := map[string]any{
		"RequestType":   "InstallApplication",
		"iTunesStoreID": iTunesStoreID,
		"Options": map[string]any{
			"PurchaseMethod": purchaseMethod,
		},
		"ManagementFlags": 5, // remove with MDM + no backup
	}
	if cfg := managedAppConfiguration(configurations); cfg != nil {
		cmdBody["Configuration"] = cfg
	}
	payload := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command":     cmdBody,
	}
	return plist.Marshal(payload)
}

// ManageExistingApplication takes over a user-installed app so Managed App Config applies.
// Uses ChangeManagementState=Managed (supervised = silent).
// Apple rejects some payloads that set both Identifier and iTunesStoreID — prefer one.
// useStoreID=true sends iTunesStoreID only; false sends Identifier (bundle) only.
func ManageExistingApplication(bundleID string, iTunesStoreID int64, configurations map[string]string, useStoreID bool) ([]byte, error) {
	bundleID = strings.TrimSpace(bundleID)
	if useStoreID && iTunesStoreID <= 0 {
		return nil, fmt.Errorf("iTunesStoreID required")
	}
	if !useStoreID && bundleID == "" {
		return nil, fmt.Errorf("bundle id required")
	}
	cmdBody := map[string]any{
		"RequestType":           "InstallApplication",
		"ChangeManagementState": "Managed",
		"ManagementFlags":       1,
	}
	if useStoreID {
		cmdBody["iTunesStoreID"] = iTunesStoreID
		cmdBody["Options"] = map[string]any{"PurchaseMethod": 1}
	} else {
		cmdBody["Identifier"] = bundleID
	}
	if cfg := managedAppConfiguration(configurations); cfg != nil {
		cmdBody["Configuration"] = cfg
	}
	payload := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command":     cmdBody,
	}
	return plist.Marshal(payload)
}

// SetApplicationConfiguration pushes Managed App Config for an already-installed app.
func SetApplicationConfiguration(bundleID string, configuration map[string]string) ([]byte, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, fmt.Errorf("bundle id required")
	}
	cfg := map[string]any{}
	for k, v := range configuration {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		cfg[k] = v
	}
	payload := map[string]any{
		"CommandUUID": uuid.NewString(),
		"Command": map[string]any{
			"RequestType": "Settings",
			"Settings": []map[string]any{
				{
					"Item":          "ApplicationConfiguration",
					"Identifier":    bundleID,
					"Configuration": cfg,
				},
			},
		},
	}
	return plist.Marshal(payload)
}

// PayloadBase64 is a helper for debugging (not used in enqueue path).
func PayloadBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
