/*
 * This file is part of Arduino Builder.
 *
 * Arduino Builder is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA  02110-1301  USA
 *
 * As a special exception, you may use this file as part of a free software
 * library without restriction.  Specifically, if other files instantiate
 * templates or use macros or inline functions from this file, or you compile
 * this file and link it with other files to produce an executable, this
 * file does not by itself cause the resulting executable to be covered by
 * the GNU General Public License.  This exception does not however
 * invalidate any other reasons why the executable file might be covered by
 * the GNU General Public License.
 *
 * Copyright 2015 Arduino LLC (http://www.arduino.cc/)
 */

package builder

import (
	"arduino.cc/builder/constants"
	"arduino.cc/builder/i18n"
	"arduino.cc/builder/types"
	"arduino.cc/builder/utils"
	"path/filepath"
)

type ContainerFindIncludes struct{}

func (s *ContainerFindIncludes) Run(context map[string]interface{}, ctx *types.Context) error {
	err := runCommand(context, ctx, &IncludesToIncludeFolders{})
	if err != nil {
		return i18n.WrapError(err)
	}

	sketchBuildPath := ctx.SketchBuildPath
	sketch := context[constants.CTX_SKETCH].(*types.Sketch)
	err = findIncludesUntilDone(context, ctx, filepath.Join(sketchBuildPath, filepath.Base(sketch.MainFile.Name)+".cpp"))
	if err != nil {
		return i18n.WrapError(err)
	}

	foldersWithSources := context[constants.CTX_FOLDERS_WITH_SOURCES_QUEUE].(*types.UniqueSourceFolderQueue)
	foldersWithSources.Push(types.SourceFolder{Folder: ctx.SketchBuildPath, Recurse: true})
	if utils.MapHas(context, constants.CTX_IMPORTED_LIBRARIES) {
		for _, library := range context[constants.CTX_IMPORTED_LIBRARIES].([]*types.Library) {
			sourceFolders := types.LibraryToSourceFolder(library)
			for _, sourceFolder := range sourceFolders {
				foldersWithSources.Push(sourceFolder)
			}
		}
	}

	err = runCommand(context, ctx, &CollectAllSourceFilesFromFoldersWithSources{})
	if err != nil {
		return i18n.WrapError(err)
	}

	sourceFilePaths := context[constants.CTX_COLLECTED_SOURCE_FILES_QUEUE].(*types.UniqueStringQueue)

	for !sourceFilePaths.Empty() {
		err = findIncludesUntilDone(context, ctx, sourceFilePaths.Pop().(string))
		if err != nil {
			return i18n.WrapError(err)
		}
		err := runCommand(context, ctx, &CollectAllSourceFilesFromFoldersWithSources{})
		if err != nil {
			return i18n.WrapError(err)
		}
	}

	err = runCommand(context, ctx, &FailIfImportedLibraryIsWrong{})
	if err != nil {
		return i18n.WrapError(err)
	}

	return nil
}

func runCommand(context map[string]interface{}, ctx *types.Context, command types.Command) error {
	PrintRingNameIfDebug(ctx, command)
	err := command.Run(context, ctx)
	if err != nil {
		return i18n.WrapError(err)
	}
	return nil
}

func findIncludesUntilDone(context map[string]interface{}, ctx *types.Context, sourceFilePath string) error {
	targetFilePath := utils.NULLFile()
	importedLibraries := context[constants.CTX_IMPORTED_LIBRARIES].([]*types.Library)
	done := false
	for !done {
		commands := []types.Command{
			&GCCPreprocRunnerForDiscoveringIncludes{SourceFilePath: sourceFilePath, TargetFilePath: targetFilePath},
			&IncludesFinderWithRegExp{ContextField: constants.CTX_GCC_MINUS_E_SOURCE},
			&IncludesToIncludeFolders{},
		}
		for _, command := range commands {
			err := runCommand(context, ctx, command)
			if err != nil {
				return i18n.WrapError(err)
			}
		}
		if len(context[constants.CTX_INCLUDES_JUST_FOUND].([]string)) == 0 {
			done = true
		} else if len(context[constants.CTX_IMPORTED_LIBRARIES].([]*types.Library)) == len(importedLibraries) {
			err := runCommand(context, ctx, &GCCPreprocRunner{TargetFileName: constants.FILE_CTAGS_TARGET_FOR_GCC_MINUS_E})
			return i18n.WrapError(err)
		}
		importedLibraries = context[constants.CTX_IMPORTED_LIBRARIES].([]*types.Library)
		context[constants.CTX_INCLUDES_JUST_FOUND] = []string{}
	}
	return nil
}
