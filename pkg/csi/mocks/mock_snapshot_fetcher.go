// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kastenhq/kubestr/pkg/csi (interfaces: SnapshotFetcher)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	snapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	types "github.com/kastenhq/kubestr/pkg/csi/types"
	v1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
)

// MockSnapshotFetcher is a mock of SnapshotFetcher interface.
type MockSnapshotFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockSnapshotFetcherMockRecorder
}

// MockSnapshotFetcherMockRecorder is the mock recorder for MockSnapshotFetcher.
type MockSnapshotFetcherMockRecorder struct {
	mock *MockSnapshotFetcher
}

// NewMockSnapshotFetcher creates a new mock instance.
func NewMockSnapshotFetcher(ctrl *gomock.Controller) *MockSnapshotFetcher {
	mock := &MockSnapshotFetcher{ctrl: ctrl}
	mock.recorder = &MockSnapshotFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSnapshotFetcher) EXPECT() *MockSnapshotFetcherMockRecorder {
	return m.recorder
}

// GetVolumeSnapshot mocks base method.
func (m *MockSnapshotFetcher) GetVolumeSnapshot(arg0 context.Context, arg1 snapshot.Snapshotter, arg2 *types.FetchSnapshotArgs) (*v1.VolumeSnapshot, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVolumeSnapshot", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v1.VolumeSnapshot)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVolumeSnapshot indicates an expected call of GetVolumeSnapshot.
func (mr *MockSnapshotFetcherMockRecorder) GetVolumeSnapshot(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVolumeSnapshot", reflect.TypeOf((*MockSnapshotFetcher)(nil).GetVolumeSnapshot), arg0, arg1, arg2)
}

// NewSnapshotter mocks base method.
func (m *MockSnapshotFetcher) NewSnapshotter() (snapshot.Snapshotter, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewSnapshotter")
	ret0, _ := ret[0].(snapshot.Snapshotter)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewSnapshotter indicates an expected call of NewSnapshotter.
func (mr *MockSnapshotFetcherMockRecorder) NewSnapshotter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewSnapshotter", reflect.TypeOf((*MockSnapshotFetcher)(nil).NewSnapshotter))
}
