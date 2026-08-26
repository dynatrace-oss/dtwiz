package extensiontest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sdkextension "github.com/dynatrace-oss/dtctl/sdk/api/extension"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"
)

type Version = sdkextension.ExtensionVersion

type MonitoringConfiguration = sdkextension.MonitoringConfiguration

type FakeExtensionAPI struct {
	Versions       []sdkextension.ExtensionVersion
	GetErr         error
	ActiveVersion  string
	ActiveErr      error
	InstallErr     error
	InstallCalled  bool
	InstallVersion string

	MonitoringConfigs []sdkextension.MonitoringConfiguration
	ListErr           error

	MonitoringConfig       *sdkextension.MonitoringConfiguration
	GetMonitoringConfigErr error

	Schema    json.RawMessage
	SchemaErr error

	CreateErr    error
	CreateCalled bool
	CreateBody   sdkextension.MonitoringConfigurationCreate

	UpdateErr      error
	UpdateCalled   bool
	UpdateConfigID string
	UpdateBody     sdkextension.MonitoringConfigurationCreate

	DeleteErr      error
	DeleteCalled   bool
	DeleteConfigID string
}

func Versions(values ...string) []sdkextension.ExtensionVersion {
	versions := make([]sdkextension.ExtensionVersion, 0, len(values))
	for _, version := range values {
		versions = append(versions, sdkextension.ExtensionVersion{Version: version})
	}
	return versions
}

func NotFound(extensionName string) error {
	return fmt.Errorf("extension %q not found", extensionName)
}

func APIError(statusCode int, message string) error {
	return httpclient.NewAPIError(statusCode, message, "")
}

func (f *FakeExtensionAPI) Get(context.Context, string) (*sdkextension.ExtensionVersionList, error) {
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	return &sdkextension.ExtensionVersionList{Items: f.Versions}, nil
}

func (f *FakeExtensionAPI) GetActiveVersion(context.Context, string) (string, error) {
	return f.ActiveVersion, f.ActiveErr
}

func (f *FakeExtensionAPI) InstallFromHub(_ context.Context, extensionName, version string) (*sdkextension.ExtensionVersion, error) {
	f.InstallCalled = true
	f.InstallVersion = version
	if f.InstallErr != nil {
		return nil, f.InstallErr
	}
	return &sdkextension.ExtensionVersion{ExtensionName: extensionName, Version: version}, nil
}

func (f *FakeExtensionAPI) ListMonitoringConfigurations(context.Context, string, string, int64) (*sdkextension.MonitoringConfigurationList, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	return &sdkextension.MonitoringConfigurationList{Items: f.MonitoringConfigs}, nil
}

func (f *FakeExtensionAPI) GetMonitoringConfiguration(context.Context, string, string) (*sdkextension.MonitoringConfiguration, error) {
	if f.GetMonitoringConfigErr != nil {
		return nil, f.GetMonitoringConfigErr
	}
	if f.MonitoringConfig == nil {
		return nil, errors.New("unexpected GetMonitoringConfiguration call")
	}
	return f.MonitoringConfig, nil
}

func (f *FakeExtensionAPI) CreateMonitoringConfiguration(_ context.Context, _ string, body sdkextension.MonitoringConfigurationCreate) (*sdkextension.MonitoringConfiguration, error) {
	f.CreateCalled = true
	f.CreateBody = body
	if f.CreateErr != nil {
		return nil, f.CreateErr
	}
	return &sdkextension.MonitoringConfiguration{ObjectID: "mon-001"}, nil
}

func (f *FakeExtensionAPI) UpdateMonitoringConfiguration(_ context.Context, _ string, configID string, body sdkextension.MonitoringConfigurationCreate) (*sdkextension.MonitoringConfiguration, error) {
	f.UpdateCalled = true
	f.UpdateConfigID = configID
	f.UpdateBody = body
	if f.UpdateErr != nil {
		return nil, f.UpdateErr
	}
	return &sdkextension.MonitoringConfiguration{ObjectID: "mon-001"}, nil
}

func (f *FakeExtensionAPI) DeleteMonitoringConfiguration(_ context.Context, _ string, configID string) error {
	f.DeleteCalled = true
	f.DeleteConfigID = configID
	return f.DeleteErr
}

func (f *FakeExtensionAPI) GetMonitoringConfigurationSchema(context.Context, string, string) (json.RawMessage, error) {
	if f.SchemaErr != nil {
		return nil, f.SchemaErr
	}
	return f.Schema, nil
}

func DecodeBody(body sdkextension.MonitoringConfigurationCreate, dst any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}
