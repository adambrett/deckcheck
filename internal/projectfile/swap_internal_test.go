package projectfile

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

// renameCall records one rename invocation observed by fakeFileOps.
type renameCall struct {
	from string
	to   string
}

// fakeFileOps builds a projectFileOps whose closures record calls and
// pop scripted errors in invocation order. No filesystem is touched.
type fakeFileOps struct {
	renameCalls  []renameCall
	renameErrs   []error
	removeCalls  []string
	backupPath   string
	tempPathErr  error
	tempPathArgs []string
}

func (f *fakeFileOps) ops(goos string) projectFileOps {
	return projectFileOps{
		goos: goos,
		rename: func(from, to string) error {
			f.renameCalls = append(f.renameCalls, renameCall{from: from, to: to})
			if len(f.renameErrs) == 0 {
				return nil
			}
			err := f.renameErrs[0]
			f.renameErrs = f.renameErrs[1:]
			return err
		},
		remove: func(path string) error {
			f.removeCalls = append(f.removeCalls, path)
			return nil
		},
		tempPath: func(path string) (string, error) {
			f.tempPathArgs = append(f.tempPathArgs, path)
			return f.backupPath, f.tempPathErr
		},
	}
}

func TestAtomicReplaceRenamesDirectlyOnNonWindows(t *testing.T) {
	// Given
	fake := &fakeFileOps{}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("linux"))

	// Then
	require.NoError(t, err)
	require.Equal(t, []renameCall{{from: "temp.deckcheck", to: "project.deckcheck"}}, fake.renameCalls)
	require.Empty(t, fake.removeCalls)
	require.Empty(t, fake.tempPathArgs)
}

func TestAtomicReplaceReturnsErrorOnNonWindowsRenameFailure(t *testing.T) {
	// Given
	renameErr := errors.New("permission denied")
	fake := &fakeFileOps{renameErrs: []error{renameErr}}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("linux"))

	// Then
	require.ErrorIs(t, err, renameErr)
	require.Len(t, fake.renameCalls, 1)
	require.Empty(t, fake.tempPathArgs)
}

func TestAtomicReplaceReturnsWindowsNonExistErrorWithoutSwap(t *testing.T) {
	// Given a Windows rename failure that is not a file-exists collision
	renameErr := errors.New("sharing violation")
	fake := &fakeFileOps{renameErrs: []error{renameErr}}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("windows"))

	// Then the error passes through and no backup swap is attempted
	require.ErrorIs(t, err, renameErr)
	require.Len(t, fake.renameCalls, 1)
	require.Empty(t, fake.tempPathArgs)
}

func TestAtomicReplaceSwapsViaBackupOnWindowsExistError(t *testing.T) {
	// Given a Windows rename refusal because the target already exists
	fake := &fakeFileOps{
		renameErrs: []error{fmt.Errorf("rename: %w", fs.ErrExist)},
		backupPath: "backup.deckcheck",
	}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("windows"))

	// Then the original moves aside, the new file installs, and the backup is removed
	require.NoError(t, err)
	require.Equal(t, []renameCall{
		{from: "temp.deckcheck", to: "project.deckcheck"},
		{from: "project.deckcheck", to: "backup.deckcheck"},
		{from: "temp.deckcheck", to: "project.deckcheck"},
	}, fake.renameCalls)
	require.Equal(t, []string{"backup.deckcheck"}, fake.removeCalls)
	require.Equal(t, []string{"project.deckcheck"}, fake.tempPathArgs)
}

func TestAtomicReplaceReturnsErrorOnBackupPathCreationFailure(t *testing.T) {
	// Given
	tempPathErr := errors.New("create temporary project file: disk full")
	fake := &fakeFileOps{
		renameErrs:  []error{fmt.Errorf("rename: %w", fs.ErrExist)},
		tempPathErr: tempPathErr,
	}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("windows"))

	// Then
	require.ErrorIs(t, err, tempPathErr)
	require.Len(t, fake.renameCalls, 1)
}

func TestAtomicReplaceReturnsErrorOnBackupRenameFailure(t *testing.T) {
	// Given the move-aside of the existing project fails
	backupErr := errors.New("access denied")
	fake := &fakeFileOps{
		renameErrs: []error{fmt.Errorf("rename: %w", fs.ErrExist), backupErr},
		backupPath: "backup.deckcheck",
	}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("windows"))

	// Then
	require.ErrorIs(t, err, backupErr)
	require.Len(t, fake.renameCalls, 2)
	require.Empty(t, fake.removeCalls)
}

func TestAtomicReplaceReturnsInstallErrorAfterSuccessfulRestore(t *testing.T) {
	// Given the install rename fails but the restore succeeds
	installErr := errors.New("install blocked")
	fake := &fakeFileOps{
		renameErrs: []error{fmt.Errorf("rename: %w", fs.ErrExist), nil, installErr, nil},
		backupPath: "backup.deckcheck",
	}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("windows"))

	// Then the install error surfaces and the original is restored in place
	require.ErrorIs(t, err, installErr)
	require.Equal(t, []renameCall{
		{from: "temp.deckcheck", to: "project.deckcheck"},
		{from: "project.deckcheck", to: "backup.deckcheck"},
		{from: "temp.deckcheck", to: "project.deckcheck"},
		{from: "backup.deckcheck", to: "project.deckcheck"},
	}, fake.renameCalls)
	require.Empty(t, fake.removeCalls)
}

func TestAtomicReplaceReportsBackupPathWhenInstallAndRestoreFail(t *testing.T) {
	// Given both the install rename and the restore rename fail
	installErr := errors.New("install blocked")
	restoreErr := errors.New("restore blocked")
	fake := &fakeFileOps{
		renameErrs: []error{fmt.Errorf("rename: %w", fs.ErrExist), nil, installErr, restoreErr},
		backupPath: "backup.deckcheck",
	}

	// When
	err := atomicReplaceProjectFileWithOps("temp.deckcheck", "project.deckcheck", fake.ops("windows"))

	// Then the message names the backup path so the user can recover the file
	require.ErrorIs(t, err, installErr)
	require.ErrorIs(t, err, restoreErr)
	require.ErrorContains(t, err, "original project is preserved at")
	require.ErrorContains(t, err, "backup.deckcheck")
	require.Empty(t, fake.removeCalls)
}
