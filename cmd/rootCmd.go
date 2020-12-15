// Copyright 2020 Kubestr Developers

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kastenhq/kubestr/pkg/csi"
	csitypes "github.com/kastenhq/kubestr/pkg/csi/types"
	"github.com/kastenhq/kubestr/pkg/fio"
	"github.com/kastenhq/kubestr/pkg/kubestr"
	"github.com/spf13/cobra"
)

var (
	output  string
	rootCmd = &cobra.Command{
		Use:   "kubestr",
		Short: "A tool to validate kubernetes storage",
		Long: `kubestr is a tool that will scan your k8s cluster
		and validate that the storage systems in place as well as run
		performance tests.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			Baseline(ctx, output)
		},
	}

	storageClass   string
	namespace      string
	containerImage string

	fioCheckerSize     string
	fioCheckerFilePath string
	fioCheckerTestName string
	fioCmd             = &cobra.Command{
		Use:   "fio",
		Short: "Runs an fio test",
		Long:  `Run an fio test`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			Fio(ctx, output, storageClass, fioCheckerSize, namespace, fioCheckerTestName, fioCheckerFilePath, containerImage)
		},
	}

	csiCheckVolumeSnapshotClass string
	csiCheckRunAsUser           int64
	csiCheckCleanup             bool
	csiCheckSkipCFSCheck        bool
	csiCheckCmd                 = &cobra.Command{
		Use:   "csicheck",
		Short: "Runs the CSI snapshot restore check",
		Long:  "Validates a CSI provisioners ability to take a snapshot of an application and restore it",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			CSICheck(ctx, output, namespace, storageClass, csiCheckVolumeSnapshotClass, csiCheckRunAsUser, containerImage, csiCheckCleanup, csiCheckSkipCFSCheck)
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "Options(json)")

	rootCmd.AddCommand(fioCmd)
	fioCmd.Flags().StringVarP(&storageClass, "storageclass", "s", "", "The name of a Storageclass. (Required)")
	_ = fioCmd.MarkFlagRequired("storageclass")
	fioCmd.Flags().StringVarP(&fioCheckerSize, "size", "z", fio.DefaultPVCSize, "The size of the volume used to run FIO.")
	fioCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace used to run FIO.")
	fioCmd.Flags().StringVarP(&fioCheckerFilePath, "fiofile", "f", "", "The path to a an fio config file.")
	fioCmd.Flags().StringVarP(&fioCheckerTestName, "testname", "t", "", "The Name of a predefined kubestr fio test. Options(default-fio)")
	fioCmd.Flags().StringVarP(&containerImage, "image", "i", "", "The container image used to create a pod.")

	rootCmd.AddCommand(csiCheckCmd)
	csiCheckCmd.Flags().StringVarP(&storageClass, "storageclass", "s", "", "The name of a Storageclass. (Required)")
	_ = csiCheckCmd.MarkFlagRequired("storageclass")
	csiCheckCmd.Flags().StringVarP(&csiCheckVolumeSnapshotClass, "volumesnapshotclass", "v", "", "The name of a VolumeSnapshotClass. (Required)")
	_ = csiCheckCmd.MarkFlagRequired("volumesnapshotclass")
	csiCheckCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace used to run the check.")
	csiCheckCmd.Flags().StringVarP(&containerImage, "image", "i", "", "The container image used to create a pod.")
	csiCheckCmd.Flags().BoolVarP(&csiCheckCleanup, "cleanup", "c", true, "Clean up the objects created by tool")
	csiCheckCmd.Flags().Int64VarP(&csiCheckRunAsUser, "runAsUser", "u", 0, "Runs the CSI check using pods as a user (int)")
	csiCheckCmd.Flags().BoolVarP(&csiCheckSkipCFSCheck, "skipCFScheck", "k", false, "Use this flag to skip validating the ability to clone a snapshot.")
}

// Execute executes the main command
func Execute() error {
	return rootCmd.Execute()
}

// Baseline executes the baseline check
func Baseline(ctx context.Context, output string) {
	p, err := kubestr.NewKubestr()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Print(kubestr.Logo)
	result := p.KubernetesChecks()
	if output == "json" {
		jsonRes, _ := json.MarshalIndent(result, "", "    ")
		fmt.Println(string(jsonRes))
		return
	}
	for _, retval := range result {
		retval.Print()
		fmt.Println()
		time.Sleep(500 * time.Millisecond)
	}

	provisionerList, err := p.ValidateProvisioners(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if output == "json" {
		jsonRes, _ := json.MarshalIndent(result, "", "    ")
		fmt.Println(string(jsonRes))
		return
	}
	fmt.Println("Available Storage Provisioners:")
	fmt.Println()
	time.Sleep(500 * time.Millisecond) // Added to introduce lag.
	for _, provisioner := range provisionerList {
		provisioner.Print()
		fmt.Println()
		time.Sleep(500 * time.Millisecond)
	}
}

// Fio executes the FIO test.
func Fio(ctx context.Context, output, storageclass, size, namespace, jobName, fioFilePath string, containerImage string) {
	cli, err := kubestr.LoadKubeCli()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fioRunner := &fio.FIOrunner{
		Cli: cli,
	}
	testName := "FIO test results"
	var result *kubestr.TestOutput
	if fioResult, err := fioRunner.RunFio(ctx, &fio.RunFIOArgs{
		StorageClass:   storageclass,
		Size:           size,
		Namespace:      namespace,
		FIOJobName:     jobName,
		FIOJobFilepath: fioFilePath,
		Image:          containerImage,
	}); err != nil {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusError, err.Error(), fioResult)
	} else {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusOK, fmt.Sprintf("\n%s\n", fioResult.Result), fioResult)
	}

	if output == "json" {
		jsonRes, _ := json.MarshalIndent(result, "", "    ")
		fmt.Println(string(jsonRes))
		return
	}
	result.Print()
}

func CSICheck(ctx context.Context, output,
	namespace string,
	storageclass string,
	volumesnapshotclass string,
	runAsUser int64,
	containerImage string,
	cleanup bool,
	skipCFScheck bool,
) {
	testName := "CSI checker test"
	kubecli, err := kubestr.LoadKubeCli()
	if err != nil {
		fmt.Printf("Failed to load kubeCLi (%s)", err.Error())
		return
	}
	dyncli, err := kubestr.LoadDynCli()
	if err != nil {
		fmt.Printf("Failed to load kubeCLi (%s)", err.Error())
		return
	}
	csiCheckRunner := &csi.SnapshotRestoreRunner{
		KubeCli: kubecli,
		DynCli:  dyncli,
	}
	var result *kubestr.TestOutput
	csiCheckResult, err := csiCheckRunner.RunSnapshotRestore(ctx, &csitypes.CSISnapshotRestoreArgs{
		StorageClass:        storageclass,
		VolumeSnapshotClass: volumesnapshotclass,
		Namespace:           namespace,
		RunAsUser:           runAsUser,
		ContainerImage:      containerImage,
		Cleanup:             cleanup,
		SkipCFSCheck:        skipCFScheck,
	})
	if err != nil {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusError, err.Error(), csiCheckResult)
	} else {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusOK, "CSI application successfully snapshotted and restored.", csiCheckResult)
	}

	if output == "json" {
		jsonRes, _ := json.MarshalIndent(result, "", "    ")
		fmt.Println(string(jsonRes))
		return
	}
	result.Print()
}
