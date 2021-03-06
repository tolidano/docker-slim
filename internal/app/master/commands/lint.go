package commands

import (
	"fmt"
	//"os"

	//"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/linter"
	"github.com/docker-slim/docker-slim/pkg/docker/linter/check"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	//"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	//v "github.com/docker-slim/docker-slim/pkg/version"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// OnLint implements the 'lint' docker-slim command
func OnLint(
	gparams *GenericParams,
	targetRef string,
	targetType string,
	doSkipBuildContext bool,
	buildContextDir string,
	doSkipDockerignore bool,
	includeCheckLabels map[string]string,
	excludeCheckLabels map[string]string,
	includeCheckIDs map[string]struct{},
	excludeCheckIDs map[string]struct{},
	doShowNoHits bool,
	doShowSnippet bool,
	ec *ExecutionContext) {
	const cmdName = command.Lint
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewLintCommand(gparams.ReportLocation)
	cmdReport.State = command.StateStarted

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=params target=%v\n", appName, cmdName, targetRef)

	/*
		do it only when targetting images
		client, err := dockerclient.New(gparams.ClientConfig)
		if err == dockerclient.ErrNoDockerInfo {
			exitMsg := "missing Docker connection info"
			if gparams.InContainer && gparams.IsDSImage {
				exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
			}
			fmt.Printf("%s[%s]: info=docker.connect.error message='%s'\n", appName, cmdName, exitMsg)
			fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
			os.Exit(ectCommon | ecNoDockerConnectInfo)
		}
		errutil.FailOn(err)
	*/
	var client *dockerapi.Client

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	cmdReport.TargetType = linter.DockerfileTargetType
	cmdReport.TargetReference = targetRef

	options := linter.Options{
		DockerfilePath:   targetRef,
		SkipBuildContext: doSkipBuildContext,
		BuildContextDir:  buildContextDir,
		SkipDockerignore: doSkipDockerignore,
		Selector: linter.CheckSelector{
			IncludeCheckLabels: includeCheckLabels,
			IncludeCheckIDs:    includeCheckIDs,
			ExcludeCheckLabels: excludeCheckLabels,
			ExcludeCheckIDs:    excludeCheckIDs,
		},
	}

	lintResults, err := linter.Execute(options)
	errutil.FailOn(err)

	cmdReport.BuildContextDir = lintResults.BuildContextDir
	cmdReport.Hits = lintResults.Hits
	cmdReport.Errors = lintResults.Errors

	printLintResults(lintResults, appName, cmdName, cmdReport, doShowNoHits, doShowSnippet)

	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = command.StateCompleted

	fmt.Printf("%s[%s]: state=done\n", appName, cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		fmt.Printf("%s[%s]: info=report file='%s'\n", appName, cmdName, cmdReport.ReportLocation())
	}
}

func printLintResults(lintResults *linter.Report,
	appName string,
	cmdName command.Type,
	cmdReport *report.LintCommand,
	doShowNoHits bool,
	doShowSnippet bool) {
	cmdReport.HitsCount = len(lintResults.Hits)
	cmdReport.NoHitsCount = len(lintResults.NoHits)
	cmdReport.ErrorsCount = len(lintResults.Errors)

	fmt.Printf("%s[%s]: info=lint.results hits=%d nohits=%d errors=%d:\n",
		appName,
		cmdName,
		cmdReport.HitsCount,
		cmdReport.NoHitsCount,
		cmdReport.ErrorsCount)

	if cmdReport.HitsCount > 0 {
		fmt.Printf("%s[%s]: info=lint.check.hits count=%d\n",
			appName, cmdName, cmdReport.HitsCount)

		for id, result := range lintResults.Hits {
			fmt.Printf("%s[%s]: info=lint.check.hit id=%s name='%s' level=%s message='%s'\n",
				appName, cmdName,
				id,
				result.Source.Name,
				result.Source.Labels[check.LabelLevel],
				result.Message)

			if len(result.Matches) > 0 {
				fmt.Printf("%s[%s]: info=lint.check.hit.matches count=%d:\n",
					appName, cmdName, len(result.Matches))

				for _, m := range result.Matches {
					var instructionInfo string
					//the match message has the instruction info already
					//if m.Instruction != nil {
					//	instructionInfo = fmt.Sprintf(" instruction(start=%d end=%d name=%s gindex=%d sindex=%d)",
					//		m.Instruction.StartLine,
					//		m.Instruction.EndLine,
					//		m.Instruction.Name,
					//		m.Instruction.GlobalIndex,
					//		m.Instruction.StageIndex)
					//}

					var stageInfo string
					if m.Stage != nil {
						stageInfo = fmt.Sprintf(" stage(index=%d name='%s')", m.Stage.Index, m.Stage.Name)
					}

					fmt.Printf("%s[%s]: info=lint.check.hit.match message='%s'%s%s\n",
						appName, cmdName, m.Message, instructionInfo, stageInfo)

					if m.Instruction != nil &&
						len(m.Instruction.RawLines) > 0 &&
						doShowSnippet {
						for idx, data := range m.Instruction.RawLines {
							fmt.Printf("%s[%s]: info=lint.check.hit.match.snippet line=%d data='%s'\n",
								appName, cmdName, idx+m.Instruction.StartLine, data)
						}
					}
				}
			}
		}
	}

	if doShowNoHits && cmdReport.NoHitsCount > 0 {
		fmt.Printf("%s[%s]: info=lint.check.nohits count=%d\n",
			appName, cmdName, cmdReport.NoHitsCount)

		for id, result := range lintResults.NoHits {
			fmt.Printf("%s[%s]: info=lint.check.nohit id=%s name='%s'\n",
				appName, cmdName, id, result.Source.Name)
		}
	}

	if cmdReport.ErrorsCount > 0 {
		fmt.Printf("%s[%s]: info=lint.check.errors count=%d: %v\n",
			appName, cmdName, cmdReport.ErrorsCount)

		for id, err := range lintResults.Errors {
			fmt.Printf("%s[%s]: info=lint.check.error id=%s message='%v'\n", appName, cmdName, id, err)
		}
	}
}
