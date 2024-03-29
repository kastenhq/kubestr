// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kastenhq/kubestr/pkg/csi (interfaces: SnapshotCreator)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	snapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	types "github.com/kastenhq/kubestr/pkg/csi/types"
	v1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v10 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockSnapshotCreator is a mock of SnapshotCreator interface.
type MockSnapshotCreator struct {
	ctrl     *gomock.Controller
	recorder *MockSnapshotCreatorMockRecorder
}

// MockSnapshotCreatorMockRecorder is the mock recorder for MockSnapshotCreator.
type MockSnapshotCreatorMockRecorder struct {
	mock *MockSnapshotCreator
}

// NewMockSnapshotCreator creates a new mock instance.
func NewMockSnapshotCreator(ctrl *gomock.Controller) *MockSnapshotCreator {
	mock := &MockSnapshotCreator{ctrl: ctrl}
	mock.recorder = &MockSnapshotCreatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSnapshotCreator) EXPECT() *MockSnapshotCreatorMockRecorder {
	return m.recorder
}

// CreateFromSourceCheck mocks base method.
func (m *MockSnapshotCreator) CreateFromSourceCheck(arg0 context.Context, arg1 snapshot.Snapshotter, arg2 *types.CreateFromSourceCheckArgs, arg3 *v10.GroupVersionForDiscovery) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateFromSourceCheck", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateFromSourceCheck indicates an expected call of CreateFromSourceCheck.
func (mr *MockSnapshotCreatorMockRecorder) CreateFromSourceCheck(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateFromSourceCheck", reflect.TypeOf((*MockSnapshotCreator)(nil).CreateFromSourceCheck), arg0, arg1, arg2, arg3)
}

// CreateSnapshot mocks base method.
func (m *MockSnapshotCreator) CreateSnapshot(arg0 context.Context, arg1 snapshot.Snapshotter, arg2 *types.CreateSnapshotArgs) (*v1.VolumeSnapshot, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateSnapshot", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v1.VolumeSnapshot)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateSnapshot indicates an expected call of CreateSnapshot.
func (mr *MockSnapshotCreatorMockRecorder) CreateSnapshot(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSnapshot", reflect.TypeOf((*MockSnapshotCreator)(nil).CreateSnapshot), arg0, arg1, arg2)
}

// NewSnapshotter mocks base method.
func (m *MockSnapshotCreator) NewSnapshotter() (snapshot.Snapshotter, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewSnapshotter")
	ret0, _ := ret[0].(snapshot.Snapshotter)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewSnapshotter indicates an expected call of NewSnapshotter.
func (mr *MockSnapshotCreatorMockRecorder) NewSnapshotter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewSnapshotter", reflect.TypeOf((*MockSnapshotCreator)(nil).NewSnapshotter))
}
