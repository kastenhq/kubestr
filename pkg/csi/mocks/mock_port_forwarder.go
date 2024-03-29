// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kastenhq/kubestr/pkg/csi (interfaces: PortForwarder)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/kastenhq/kubestr/pkg/csi/types"
	rest "k8s.io/client-go/rest"
)

// MockPortForwarder is a mock of PortForwarder interface.
type MockPortForwarder struct {
	ctrl     *gomock.Controller
	recorder *MockPortForwarderMockRecorder
}

// MockPortForwarderMockRecorder is the mock recorder for MockPortForwarder.
type MockPortForwarderMockRecorder struct {
	mock *MockPortForwarder
}

// NewMockPortForwarder creates a new mock instance.
func NewMockPortForwarder(ctrl *gomock.Controller) *MockPortForwarder {
	mock := &MockPortForwarder{ctrl: ctrl}
	mock.recorder = &MockPortForwarderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPortForwarder) EXPECT() *MockPortForwarderMockRecorder {
	return m.recorder
}

// FetchRestConfig mocks base method.
func (m *MockPortForwarder) FetchRestConfig() (*rest.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchRestConfig")
	ret0, _ := ret[0].(*rest.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchRestConfig indicates an expected call of FetchRestConfig.
func (mr *MockPortForwarderMockRecorder) FetchRestConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchRestConfig", reflect.TypeOf((*MockPortForwarder)(nil).FetchRestConfig))
}

// PortForwardAPod mocks base method.
func (m *MockPortForwarder) PortForwardAPod(arg0 *types.PortForwardAPodRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PortForwardAPod", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// PortForwardAPod indicates an expected call of PortForwardAPod.
func (mr *MockPortForwarderMockRecorder) PortForwardAPod(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PortForwardAPod", reflect.TypeOf((*MockPortForwarder)(nil).PortForwardAPod), arg0)
}
