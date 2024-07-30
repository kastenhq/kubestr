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

	"github.com/kastenhq/kubestr/pkg/block"
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
		Args:         cobra.ExactArgs(0),
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
	fioNodeSelector    map[string]string
	fioCheckerFilePath string
	fioCheckerTestName string
	fioCmd             = &cobra.Command{
		Use:   "fio",
		Short: "Runs an fio test",
		Long:  `Run an fio test`,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return Fio(ctx, output, outfile, storageClass, fioCheckerSize, namespace, fioNodeSelector, fioCheckerTestName, fioCheckerFilePath, containerImage)
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
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return CSICheck(ctx, output, outfile, namespace, storageClass, csiCheckVolumeSnapshotClass, csiCheckRunAsUser, containerImage, csiCheckCleanup, csiCheckSkipCFSCheck)
		},
	}

	browseLocalPort int
	browseCmd       = &cobra.Command{
		Use:        "browse",
		Short:      "Browse the contents of PVC or VolumeSnapshot",
		Long:       "Browse the contents of a CSI provisioned PVC or a CSI provisioned VolumeSnapshot.",
		Deprecated: "use 'browse pvc' instead",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return browsePvcCmd.RunE(cmd, args)
		},
	}

	browsePvcCmd = &cobra.Command{
		Use:   "pvc [PVC name]",
		Short: "Browse the contents of a CSI PVC via file browser",
		Long:  "Browse the contents of a CSI provisioned PVC by cloning the volume and mounting it with a file browser.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return CsiPvcBrowse(context.Background(), args[0],
				namespace,
				csiCheckVolumeSnapshotClass,
				csiCheckRunAsUser,
				browseLocalPort,
			)
		},
	}

	browseSnapshotCmd = &cobra.Command{
		Use:   "snapshot [Snapshot name]",
		Short: "Browse the contents of a CSI VolumeSnapshot via file browser",
		Long:  "Browse the contents of a CSI provisioned VolumeSnapshot by cloning the volume and mounting it with a file browser.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return CsiSnapshotBrowse(context.Background(), args[0],
				namespace,
				storageClass,
				csiCheckRunAsUser,
				browseLocalPort,
			)
		},
	}

	blockMountRunAsUser          int64
	blockMountCleanup            bool
	blockMountCleanupOnly        bool
	blockMountWaitTimeoutSeconds uint32
	blockMountPVCSize            string
	blockMountCmd                = &cobra.Command{
		Use:   "blockmount",
		Short: "Checks if a storage class supports block volumes",
		Long: `Checks if volumes provisioned by a storage class can be mounted in block mode.

The checker works as follows:
- It dynamically provisions a volume of the given storage class.
- It then launches a pod with the volume mounted as a block device.
- If the pod is successfully created then the test passes.
- If the pod fails or times out then the test fails.

In case of failure, re-run the checker with the "-c=false" flag and examine the
failed PVC and Pod: it may be necessary to adjust the default values used for
the PVC size, the pod wait timeout, etc. Clean up the failed resources by
running the checker with the "--cleanup-only" flag.
`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			checkerArgs := block.BlockMountCheckerArgs{
				StorageClass:          storageClass,
				Namespace:             namespace,
				Cleanup:               blockMountCleanup,
				RunAsUser:             blockMountRunAsUser,
				ContainerImage:        containerImage,
				K8sObjectReadyTimeout: (time.Second * time.Duration(blockMountWaitTimeoutSeconds)),
				PVCSize:               blockMountPVCSize,
			}
			return BlockMountCheck(ctx, output, outfile, blockMountCleanupOnly, checkerArgs)
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "Options(json)")
	rootCmd.PersistentFlags().StringVarP(&outfile, "outfile", "e", "", "The file where test results will be written")

	rootCmd.AddCommand(fioCmd)
	fioCmd.Flags().StringVarP(&storageClass, "storageclass", "s", "", "The name of a Storageclass. (Required)")
	_ = fioCmd.MarkFlagRequired("storageclass")
	fioCmd.Flags().StringVarP(&fioCheckerSize, "size", "z", fio.DefaultPVCSize, "The size of the volume used to run FIO. Note that the FIO job definition is not scaled accordingly.")
	fioCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace used to run FIO.")
	fioCmd.Flags().StringToStringVarP(&fioNodeSelector, "nodeselector", "N", map[string]string{}, "Node selector applied to pod.")
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
	csiCheckCmd.Flags().Int64VarP(&csiCheckRunAsUser, "runAsUser", "u", 0, "Runs the CSI check pod with the specified user ID (int)")
	csiCheckCmd.Flags().BoolVarP(&csiCheckSkipCFSCheck, "skipCFScheck", "k", false, "Use this flag to skip validating the ability to clone a snapshot.")

	rootCmd.AddCommand(browseCmd)
	browseCmd.Flags().StringVarP(&csiCheckVolumeSnapshotClass, "volumesnapshotclass", "v", "", "The name of a VolumeSnapshotClass. (Required)")
	_ = browseCmd.MarkFlagRequired("volumesnapshotclass")
	browseCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace of the PersistentVolumeClaim.")
	browseCmd.Flags().Int64VarP(&csiCheckRunAsUser, "runAsUser", "u", 0, "Runs the inspector pod as a user (int)")
	browseCmd.Flags().IntVarP(&browseLocalPort, "localport", "l", 8080, "The local port to expose the inspector")

	browseCmd.AddCommand(browsePvcCmd)
	browsePvcCmd.Flags().StringVarP(&csiCheckVolumeSnapshotClass, "volumesnapshotclass", "v", "", "The name of a VolumeSnapshotClass. (Required)")
	_ = browsePvcCmd.MarkFlagRequired("volumesnapshotclass")
	browsePvcCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace of the PersistentVolumeClaim.")
	browsePvcCmd.Flags().Int64VarP(&csiCheckRunAsUser, "runAsUser", "u", 0, "Runs the inspector pod as a user (int)")
	browsePvcCmd.Flags().IntVarP(&browseLocalPort, "localport", "l", 8080, "The local port to expose the inspector")

	browseCmd.AddCommand(browseSnapshotCmd)
	browseSnapshotCmd.Flags().StringVarP(&storageClass, "storageclass", "s", "", "The name of a StorageClass. (Required)")
	_ = browseSnapshotCmd.MarkFlagRequired("storageclass")
	browseSnapshotCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace of the VolumeSnapshot.")
	browseSnapshotCmd.Flags().Int64VarP(&csiCheckRunAsUser, "runAsUser", "u", 0, "Runs the inspector pod as a user (int)")
	browseSnapshotCmd.Flags().IntVarP(&browseLocalPort, "localport", "l", 8080, "The local port to expose the inspector")

	rootCmd.AddCommand(blockMountCmd)
	blockMountCmd.Flags().StringVarP(&storageClass, "storageclass", "s", "", "The name of a StorageClass. (Required)")
	_ = blockMountCmd.MarkFlagRequired("storageclass")
	blockMountCmd.Flags().StringVarP(&namespace, "namespace", "n", fio.DefaultNS, "The namespace used to run the check.")
	blockMountCmd.Flags().StringVarP(&containerImage, "image", "i", "", "The container image used to create a pod.")
	blockMountCmd.Flags().BoolVarP(&blockMountCleanup, "cleanup", "c", true, "Clean up the objects created by the check.")
	blockMountCmd.Flags().BoolVarP(&blockMountCleanupOnly, "cleanup-only", "", false, "Do not run the checker, but just clean up resources left from a previous invocation.")
	blockMountCmd.Flags().Int64VarP(&blockMountRunAsUser, "runAsUser", "u", 0, "Runs the block mount check pod with the specified user ID (int)")
	blockMountCmd.Flags().Uint32VarP(&blockMountWaitTimeoutSeconds, "wait-timeout", "w", 60, "Max time in seconds to wait for the check pod to become ready")
	blockMountCmd.Flags().StringVarP(&blockMountPVCSize, "pvc-size", "", "1Gi", "The size of the provisioned PVC.")
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
func Fio(ctx context.Context, output, outfile, storageclass, size, namespace string, nodeSelector map[string]string, jobName, fioFilePath string, containerImage string) error {
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
		NodeSelector:   nodeSelector,
		FIOJobName:     jobName,
		FIOJobFilepath: fioFilePath,
		Image:          containerImage,
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

func CsiSnapshotBrowse(ctx context.Context,
	snapshotName string,
	namespace string,
	storageClass string,
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
	browseRunner := &csi.SnapshotBrowseRunner{
		KubeCli: kubecli,
		DynCli:  dyncli,
	}
	err = browseRunner.RunSnapshotBrowse(ctx, &csitypes.SnapshotBrowseArgs{
		SnapshotName:     snapshotName,
		Namespace:        namespace,
		StorageClassName: storageClass,
		RunAsUser:        runAsUser,
		LocalPort:        localPort,
	})
	if err != nil {
		fmt.Printf("Failed to run Snapshot browser (%s)\n", err.Error())
	}
	return err
}

func BlockMountCheck(ctx context.Context, output, outfile string, cleanupOnly bool, checkerArgs block.BlockMountCheckerArgs) error {
	kubecli, err := kubestr.LoadKubeCli()
	if err != nil {
		fmt.Printf("Failed to load kubeCli (%s)", err.Error())
		return err
	}
	checkerArgs.KubeCli = kubecli

	dyncli, err := kubestr.LoadDynCli()
	if err != nil {
		fmt.Printf("Failed to load dynCli (%s)", err.Error())
		return err
	}
	checkerArgs.DynCli = dyncli

	blockMountTester, err := block.NewBlockMountChecker(checkerArgs)
	if err != nil {
		fmt.Printf("Failed to initialize BlockMounter (%s)", err.Error())
		return err
	}

	if cleanupOnly {
		blockMountTester.Cleanup()
		return nil
	}

	var (
		testName = "Block VolumeMode test"
		result   *kubestr.TestOutput
	)

	mountResult, err := blockMountTester.Mount(ctx)
	if err != nil {
		if !checkerArgs.Cleanup {
			fmt.Printf("Warning: Resources may not have been released. Rerun with the additional --cleanup-only flag.\n")
		}
		result = kubestr.MakeTestOutput(testName, kubestr.StatusError, fmt.Sprintf("StorageClass (%s) does not appear to support Block VolumeMode", checkerArgs.StorageClass), mountResult)
	} else {
		result = kubestr.MakeTestOutput(testName, kubestr.StatusOK, fmt.Sprintf("StorageClass (%s) supports Block VolumeMode", checkerArgs.StorageClass), mountResult)
	}

	var wrappedResult = []*kubestr.TestOutput{result}
	if !PrintAndJsonOutput(wrappedResult, output, outfile) {
		result.Print()
	}

	return err
}
