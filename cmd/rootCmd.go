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
	"os"
	"time"

	"github.com/kastenhq/kubestr/pkg/csi"
	csitypes "github.com/kastenhq/kubestr/pkg/csi/types"
	"github.com/kastenhq/kubestr/pkg/fio"
	"github.com/kastenhq/kubestr/pkg/kubestr"
	"github.com/spf13/cobra"
)

var (
	output  string
	outfile string
	rootCmd = &cobra.Command{
		Use:   "kubestr",
		Short: "A tool to validate kubernetes storage",
		Long: `kubestr is a tool that will scan your k8s cluster
		and validate that the storage systems in place as well as run
		performance tests.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return Baseline(ctx, output)
		},
	}

	storageClass   string
	namespace      string
	containerImage string

	fioCheckerSize     string
	fioCheckerFilePath string
	fioCheckerTestName string
	fioNodeAffinities  []string
	fioTolerations     []string
	fioCmd             = &cobra.Command{
		Use:   "fio",
		Short: "Runs an fio test",
		Long:  `Run an fio test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return Fio(ctx, output, outfile, storageClass, fioCheckerSize, namespace, fioCheckerTestName, fioCheckerFilePath, containerImage, fioNodeAffinities, fioTolerations)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return CSICheck(ctx, output, outfile, namespace, storageClass, csiCheckVolumeSnapshotClass, csiCheckRunAsUser, containerImage, csiCheckCleanup, csiCheckSkipCFSCheck)
		},
	}

	pvcBrowseLocalPort int
	pvcBrowseCmd       = &cobra.Command{
		Use:   "browse [PVC name]",
		Short: "Browse the contents of a CSI PVC via file browser",
		Args:  cobra.ExactArgs(1),
		Long:  "Browse the contents of a CSI provisioned PVC by cloning the volume and mounting it with a file browser.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return CsiPvcBrowse(context.Background(), args[0],
				namespace,
				csiCheckVolumeSnapshotClass,
				csiCheckRunAsUser,
				pvcBrowseLocalPort,
			)
		},
	}
)

func init() {
	var defaultAffinities []string = make([]string, 0)
	var defaultTolerations []string = make([]string, 0)

	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "Options(json)")
	rootCmd.PersistentFlags().StringVarP(&outfile, "outfile", "e", "", "The file where test results will be written")

	rootCmd.AddCommand(fioCmd)
	fioCmd.Flags().StringVarP(&storageClass, "storageclass", "s", "", "The name of a Storageclass. (Required)")
	_ = fioCmd.MarkFlagRequired("storageclass")
	fioCmd.Flags().StringVarP(&fioCheckerSize, "size", "z", fio.DefaultPVCSize, "The size of the volume used to run FIO. Note that the FIO job definition is not scaled accordingly.")
	fioCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace used to run FIO.")
	fioCmd.Flags().StringVarP(&fioCheckerFilePath, "fiofile", "f", "", "The path to a an fio config file.")
	fioCmd.Flags().StringVarP(&fioCheckerTestName, "testname", "t", "", "The Name of a predefined kubestr fio test. Options(default-fio)")
	fioCmd.Flags().StringVarP(&containerImage, "image", "i", "", "The container image used to create a pod.")
	fioCmd.Flags().StringArrayVarP(&fioNodeAffinities, "node-affinity", "l", defaultAffinities, "The label(s) and optional value(s) to use in the FIO pod spec node affinities.")
	fioCmd.Flags().StringArrayVarP(&fioTolerations, "toleration", "T", defaultTolerations, "The toleration key(s) and optional value(s) to use in the FIO pod spec tolerations.")

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

	rootCmd.AddCommand(pvcBrowseCmd)
	pvcBrowseCmd.Flags().StringVarP(&csiCheckVolumeSnapshotClass, "volumesnapshotclass", "v", "", "The name of a VolumeSnapshotClass. (Required)")
	_ = pvcBrowseCmd.MarkFlagRequired("volumesnapshotclass")
	pvcBrowseCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace of the PersistentVolumeClaim.")
	pvcBrowseCmd.Flags().Int64VarP(&csiCheckRunAsUser, "runAsUser", "u", 0, "Runs the inspector pod as a user (int)")
	pvcBrowseCmd.Flags().IntVarP(&pvcBrowseLocalPort, "localport", "l", 8080, "The local port to expose the inspector")
}

// Execute executes the main command
func Execute() error {
	return rootCmd.Execute()
}

// Baseline executes the baseline check
func Baseline(ctx context.Context, output string) error {
	p, err := kubestr.NewKubestr()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fmt.Print(kubestr.Logo)
	result := p.KubernetesChecks()

	if PrintAndJsonOutput(result, output, outfile) {
		return err
	}

	for _, retval := range result {
		retval.Print()
		fmt.Println()
		time.Sleep(500 * time.Millisecond)
	}

	provisionerList, err := p.ValidateProvisioners(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	fmt.Println("Available Storage Provisioners:")
	fmt.Println()
	time.Sleep(500 * time.Millisecond) // Added to introduce lag.
	for _, provisioner := range provisionerList {
		provisioner.Print()
		fmt.Println()
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// PrintAndJsonOutput Print JSON output to stdout and to file if arguments say so
// Returns whether we have generated output or JSON
func PrintAndJsonOutput(result []*kubestr.TestOutput, output string, outfile string) bool {
	if output == "json" {
		jsonRes, _ := json.MarshalIndent(result, "", "    ")
		if len(outfile) > 0 {
			err := os.WriteFile(outfile, jsonRes, 0666)
			if err != nil {
				fmt.Println("Error writing output:", err.Error())
				os.Exit(2)
			}
		} else {
			fmt.Println(string(jsonRes))
		}
		return true
	}
	return false
}

// Fio executes the FIO test.
func Fio(ctx context.Context, output, outfile, storageclass, size, namespace, jobName, fioFilePath string, containerImage string, nodeAffinities []string, tolerations []string) error {
	cli, err := kubestr.LoadKubeCli()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fioRunner := &fio.FIOrunner{
		Cli: cli,
	}
	testName := "FIO test results"
	var result *kubestr.TestOutput
	fioResult, err := fioRunner.RunFio(ctx, &fio.RunFIOArgs{
		StorageClass:   storageclass,
		Size:           size,
		Namespace:      namespace,
		FIOJobName:     jobName,
		FIOJobFilepath: fioFilePath,
		Image:          containerImage,
		NodeAffinities: nodeAffinities,
		Tolerations:    tolerations,
	})
	if err != nil {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusError, err.Error(), fioResult)
	} else {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusOK, fmt.Sprintf("\n%s", fioResult.Result.Print()), fioResult)
	}
	var wrappedResult = []*kubestr.TestOutput{result}
	if !PrintAndJsonOutput(wrappedResult, output, outfile) {
		result.Print()
	}
	return err
}

func CSICheck(ctx context.Context, output, outfile,
	namespace string,
	storageclass string,
	volumesnapshotclass string,
	runAsUser int64,
	containerImage string,
	cleanup bool,
	skipCFScheck bool,
) error {
	testName := "CSI checker test"
	kubecli, err := kubestr.LoadKubeCli()
	if err != nil {
		fmt.Printf("Failed to load kubeCli (%s)", err.Error())
		return err
	}
	dyncli, err := kubestr.LoadDynCli()
	if err != nil {
		fmt.Printf("Failed to load dynCli (%s)", err.Error())
		return err
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

	var wrappedResult = []*kubestr.TestOutput{result}
	if !PrintAndJsonOutput(wrappedResult, output, outfile) {
		result.Print()
	}
	return err
}

func CsiPvcBrowse(ctx context.Context,
	pvcName string,
	namespace string,
	volumeSnapshotClass string,
	runAsUser int64,
	localPort int,
) error {
	kubecli, err := kubestr.LoadKubeCli()
	if err != nil {
		fmt.Printf("Failed to load kubeCli (%s)", err.Error())
		return err
	}
	dyncli, err := kubestr.LoadDynCli()
	if err != nil {
		fmt.Printf("Failed to load dynCli (%s)", err.Error())
		return err
	}
	browseRunner := &csi.PVCBrowseRunner{
		KubeCli: kubecli,
		DynCli:  dyncli,
	}
	err = browseRunner.RunPVCBrowse(ctx, &csitypes.PVCBrowseArgs{
		PVCName:             pvcName,
		Namespace:           namespace,
		VolumeSnapshotClass: volumeSnapshotClass,
		RunAsUser:           runAsUser,
		LocalPort:           localPort,
	})
	if err != nil {
		fmt.Printf("Failed to run PVC browser (%s)\n", err.Error())
	}
	return err
}
