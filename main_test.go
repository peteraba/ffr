package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	l = logger{
		silent: true,
	}
}

func createExampleVideo(t *testing.T, filePath string) {
	_, err := exec(fmt.Sprintf(`ffmpeg -f lavfi -i testsrc=duration=10:size=320x240:rate=30 "%s"`, filePath))
	require.NoError(t, err)
}

func cleanUp(t *testing.T, want, need []string) {
	for _, fileName := range want {
		assert.FileExists(t, fileName)

		err := os.Remove(fileName)
		require.NoError(t, err)
	}
	for _, fileName := range need {
		_ = os.Remove(fileName)
	}
}

func Test_addNumber(t *testing.T) {
	type args struct {
		filePath          string
		regularExpression string
		numberToAdd       int64
		regexpGroup       int
		skipFinds         int
		maxCount          int
		forceOverwrite    bool
		dryRun            bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:          "foo-1bar.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-3bar.txt"},
		},
		{
			name: "regular expression",
			need: []string{"foo-bb1bb-1bar.txt"},
			args: args{
				filePath:          "foo-bb1bb-1bar.txt",
				regularExpression: "\\w{2}(\\d+)\\w{2}",
				numberToAdd:       2,
				regexpGroup:       1,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-bb3bb-1bar.txt"},
		},
		{
			name: "change all",
			need: []string{"foo-1bar-2baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-3bar-4baz.txt"},
		},
		{
			name: "skip finds",
			need: []string{"foo-1bar-2baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         1,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-1bar-4baz.txt"},
		},
		{
			name: "change only the first",
			need: []string{"foo-1bar-2baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          1,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-3bar-2baz.txt"},
		},
		{
			name: "change only the second",
			need: []string{"foo-1bar-2baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         1,
				maxCount:          1,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-1bar-4baz.txt"},
		},
		{
			name: "force overwrite",
			need: []string{"foo-1bar-2baz.txt", "foo-1bar-4baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         1,
				maxCount:          1,
				forceOverwrite:    true,
				dryRun:            false,
			},
			want: []string{"foo-1bar-4baz.txt"},
		},
		{
			name: "dry-run",
			need: []string{"foo-1bar-2baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				numberToAdd:       2,
				regexpGroup:       0,
				skipFinds:         1,
				maxCount:          1,
				forceOverwrite:    true,
				dryRun:            true,
			},
			want: []string{"foo-1bar-2baz.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)

			// execute
			result := addNumber(fi, tt.args.regularExpression, tt.args.numberToAdd, tt.args.regexpGroup, tt.args.skipFinds, tt.args.maxCount, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_concat(t *testing.T) {
	type args struct {
		parts     []string
		skip      int
		newPart   string
		ext       string
		separator string
	}

	panicTests := []struct {
		name string
		args args
	}{
		{
			name: "empty-parts",
			args: args{
				parts:     []string{},
				skip:      1,
				newPart:   "quix",
				ext:       ".txt",
				separator: "-",
			},
		},
		{
			name: "non-empty-parts",
			args: args{
				parts:     []string{"foo", "bar", "baz"},
				skip:      4,
				newPart:   "quix",
				ext:       ".txt",
				separator: "-",
			},
		},
	}

	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() { concat(tt.args.parts, tt.args.skip, tt.args.newPart, tt.args.ext, tt.args.separator) })
		})
	}

	successTests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no skip",
			args: args{
				parts:     []string{"foo", "bar", "baz"},
				skip:      0,
				newPart:   "quix",
				ext:       ".txt",
				separator: "-",
			},
			want: "quix-foo-bar-baz.txt",
		},
		{
			name: "skip to middle",
			args: args{
				parts:     []string{"foo", "bar", "baz"},
				skip:      2,
				newPart:   "quix",
				ext:       ".txt",
				separator: "-",
			},
			want: "foo-bar-quix-baz.txt",
		},
		{
			name: "skip to last",
			args: args{
				parts:     []string{"foo", "bar", "baz"},
				skip:      3,
				newPart:   "quix",
				ext:       ".txt",
				separator: "-",
			},
			want: "foo-bar-baz-quix.txt",
		},
	}
	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := concat(tt.args.parts, tt.args.skip, tt.args.newPart, tt.args.ext, tt.args.separator); got != tt.want {
				t.Errorf("concat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deleteParts(t *testing.T) {
	type args struct {
		filePath       string
		partsToDelete  []int
		fromBack       bool
		forceOverwrite bool
		dryRun         bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "delete first",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:       "foo-1bar.txt",
				partsToDelete:  []int{1},
				fromBack:       false,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"1bar.txt"},
		},
		{
			name: "delete second",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:       "foo-1bar.txt",
				partsToDelete:  []int{2},
				fromBack:       false,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo.txt"},
		},
		{
			name: "delete last and third last",
			need: []string{"foo-bar-baz-quix-quiix.txt"},
			args: args{
				filePath:       "foo-bar-baz-quix-quiix.txt",
				partsToDelete:  []int{1, 3},
				fromBack:       true,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-bar-quix.txt"},
		},
		{
			name: "dry run",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:       "foo-1bar.txt",
				partsToDelete:  []int{1},
				fromBack:       true,
				forceOverwrite: false,
				dryRun:         true,
			},
			want: []string{"foo-1bar.txt"},
		},
		{
			name: "force overwrite",
			need: []string{"foo-1bar.txt", "foo.txt"},
			args: args{
				filePath:       "foo-1bar.txt",
				partsToDelete:  []int{1},
				fromBack:       true,
				forceOverwrite: true,
				dryRun:         false,
			},
			want: []string{"foo.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)

			// execute
			result := deleteParts(fi, tt.args.partsToDelete, tt.args.fromBack, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_deleteRegexp(t *testing.T) {
	type args struct {
		filePath          string
		regularExpression string
		regexpGroup       int
		skipFinds         int
		maxCount          int
		forceOverwrite    bool
		dryRun            bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:          "foo-1bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo.txt"},
		},
		{
			name: "multiple",
			need: []string{"foo-1bar-2bar.txt"},
			args: args{
				filePath:          "foo-1bar-2bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo.txt"},
		},
		{
			name: "multiple with in-between",
			need: []string{"foo-1bar-BAZ-2bar.txt"},
			args: args{
				filePath:          "foo-1bar-BAZ-2bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-BAZ.txt"},
		},
		{
			name: "skip 1",
			need: []string{"foo-1bar-BAZ-2bar.txt"},
			args: args{
				filePath:          "foo-1bar-BAZ-2bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         1,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-1bar-BAZ.txt"},
		},
		{
			name: "max 1",
			need: []string{"foo-1bar-BAZ-2bar.txt"},
			args: args{
				filePath:          "foo-1bar-BAZ-2bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          1,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-BAZ-2bar.txt"},
		},
		{
			name: "force overwrite",
			need: []string{"foo-1bar.txt", "foo.txt"},
			args: args{
				filePath:          "foo-1bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    true,
				dryRun:            false,
			},
			want: []string{"foo.txt"},
		},
		{
			name: "dry run",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:          "foo-1bar.txt",
				regularExpression: "",
				regexpGroup:       0,
				skipFinds:         0,
				maxCount:          0,
				forceOverwrite:    false,
				dryRun:            true,
			},
			want: []string{"foo-1bar.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)

			// execute
			result := deleteRegexp(fi, tt.args.regularExpression, tt.args.regexpGroup, tt.args.skipFinds, tt.args.maxCount, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_exec(t *testing.T) {
	type args struct {
		command string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "default",
			args: args{
				command: "echo 'hello'",
			},
			want: "hello\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exec(tt.args.command)
			require.NoError(t, err)
			assert.Equalf(t, tt.want, got, "exec(%v)", tt.args.command)
		})
	}
}

func Test_insertBefore(t *testing.T) {
	type args struct {
		filePath          string
		regularExpression string
		skipDuplicate     bool
		skipDashPrefix    bool
		insertText        string
		forceOverwrite    bool
		dryRun            bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:          "foo-1bar.txt",
				regularExpression: "",
				skipDashPrefix:    false,
				insertText:        "FOO",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-FOO-1bar.txt"},
		},
		{
			name: "at the beginning",
			need: []string{"foo-1bar.txt"},
			args: args{
				filePath:          "foo-1bar.txt",
				regularExpression: "foo",
				skipDashPrefix:    true,
				insertText:        "FOO",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"FOO-foo-1bar.txt"},
		},
		{
			name: "with regular expression",
			need: []string{"foo-barzan.txt"},
			args: args{
				filePath:          "foo-barzan.txt",
				regularExpression: "bar[a-z]+",
				skipDashPrefix:    false,
				insertText:        "FOO",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-FOO-barzan.txt"},
		},
		{
			name: "not found",
			need: []string{"foo.txt"},
			args: args{
				filePath:          "foo.txt",
				regularExpression: "",
				skipDashPrefix:    false,
				insertText:        "FOO",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-FOO.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)
			result := insertBefore(fi, tt.args.regularExpression, tt.args.insertText, tt.args.skipDuplicate, tt.args.skipDashPrefix, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_insertDimensionsBefore(t *testing.T) {
	t.Run("does not overwrite by default", func(t *testing.T) {
		t.Parallel()

		forceOverwrite := false
		dryRun := false
		expectedFile := "1-320x240.mp4"

		vidPath := "1.mp4"
		want := []string{vidPath, expectedFile}
		need := []string{vidPath, expectedFile}

		defer cleanUp(t, want, need)

		var err error

		// setup
		for _, filePath := range need {
			require.NoFileExists(t, filePath)
			createExampleVideo(t, filePath)
		}

		allWritten := time.Now()

		// execute
		fi, err := os.Stat(vidPath)
		require.NoError(t, err)
		result := insertDimensionsBefore(fi, "", false, true, forceOverwrite, dryRun)

		// assert
		assert.NoError(t, result)
		for _, fileName := range want {
			assert.FileExists(t, fileName)
		}

		// same as before
		fi2, err := os.Stat(expectedFile)
		assert.Greater(t, allWritten.UnixNano(), fi2.ModTime().UnixNano())
	})

	type args struct {
		filePath          string
		regularExpression string
		skipDuplicate     bool
		skipDashPrefix    bool
		insertText        string
		forceOverwrite    bool
		dryRun            bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo.mp4"},
			args: args{
				filePath:          "foo.mp4",
				regularExpression: "",
				skipDuplicate:     false,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-320x240.mp4"},
		},
		{
			name: "duplicate ok",
			need: []string{"foo-320x240.mp4"},
			args: args{
				filePath:          "foo-320x240.mp4",
				regularExpression: "",
				skipDuplicate:     false,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-320x240-320x240.mp4"},
		},
		{
			name: "duplicate not ok",
			need: []string{"foo-320x240.mp4"},
			args: args{
				filePath:          "foo-320x240.mp4",
				regularExpression: "",
				skipDuplicate:     true,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-320x240.mp4"},
		},
		{
			name: "default with extra infoAll",
			need: []string{"foo-1bar.mp4"},
			args: args{
				filePath:          "foo-1bar.mp4",
				regularExpression: "",
				skipDuplicate:     false,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-320x240-1bar.mp4"},
		},
		{
			name: "default with regular expression",
			need: []string{"foo-BAR.mp4"},
			args: args{
				filePath:          "foo-BAR.mp4",
				regularExpression: "BAR",
				skipDuplicate:     false,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-320x240-BAR.mp4"},
		},
		{
			name: "number in the middle",
			need: []string{"foo4bar.mp4"},
			args: args{
				filePath:          "foo4bar.mp4",
				regularExpression: "",
				skipDuplicate:     false,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo4bar-320x240.mp4"},
		},
		{
			name: "extreme tendencies",
			need: []string{"foobar-barbaz-Foo bar baz 4 quix-0cut-1ffc-bar-baz-2foo-baz.mp4"},
			args: args{
				filePath:          "foobar-barbaz-Foo bar baz 4 quix-0cut-1ffc-bar-baz-2foo-baz.mp4",
				regularExpression: "",
				skipDuplicate:     false,
				skipDashPrefix:    false,
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foobar-barbaz-Foo bar baz 4 quix-0cut-1ffc-bar-baz-320x240-2foo-baz.mp4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				require.NoFileExists(t, filePath)
				createExampleVideo(t, filePath)
			}

			// execute
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)
			result := insertDimensionsBefore(fi, tt.args.regularExpression, tt.args.skipDuplicate, tt.args.skipDashPrefix, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_getFileInfoList(t *testing.T) {
	type args struct {
		filePaths     []string
		backwardsFlag bool
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "forward",
			args: args{
				filePaths:     []string{"foo.txt", "bar.txt"},
				backwardsFlag: false,
			},
			want: []string{"foo.txt", "bar.txt"},
		},
		{
			name: "backward",
			args: args{
				filePaths:     []string{"foo.txt", "bar.txt"},
				backwardsFlag: true,
			},
			want: []string{"bar.txt", "foo.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, nil)

			var err error

			// setup
			for _, filePath := range tt.args.filePaths {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			result := getFileInfoList(tt.args.filePaths, tt.args.backwardsFlag)

			// assert
			for i, fi := range result {
				assert.Equal(t, tt.want[i], fi.Name())
			}
		})
	}
}

func Test_keyFrames(t *testing.T) {
	type args struct {
		filePath          string
		regularExpression string
		insertText        string
		forceOverwrite    bool
	}
	tests := []struct {
		name       string
		need       []string
		args       args
		wantOutput string
		want       []string
	}{
		{
			name: "default",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
			},
			wantOutput: "indexes: 0.0, 8.3...",
			want:       []string{"foo.mp4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				_ = os.Remove(filePath)
				createExampleVideo(t, filePath)
			}
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)

			// execute
			result := keyFrames(fi)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
			assert.Contains(t, l.history, tt.wantOutput)
		})
	}
}

func Test_mergeParts(t *testing.T) {
	type args struct {
		filePath          string
		regularExpression string
		deleteText        string
		forceOverwrite    bool
		dryRun            bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo-1bar-2baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz.txt",
				regularExpression: "",
				deleteText:        "",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-3bar-baz.txt"},
		},
		{
			name: "multiple",
			need: []string{"foo-1bar-2baz-3quix.txt"},
			args: args{
				filePath:          "foo-1bar-2baz-3quix.txt",
				regularExpression: "",
				deleteText:        "",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-6bar-baz-quix.txt"},
		},
		{
			name: "multiple with regexp",
			need: []string{"foo-1bar-2baz-3baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz-3baz.txt",
				regularExpression: "(baz)",
				deleteText:        "",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-1bar-5baz-baz.txt"},
		},
		{
			name: "multiple with regexp and delete",
			need: []string{"foo-1bar-2baz-3baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz-3baz.txt",
				regularExpression: "bar?z?",
				deleteText:        "-baz-baz",
				forceOverwrite:    false,
				dryRun:            false,
			},
			want: []string{"foo-6bar.txt"},
		},
		{
			name: "dry run",
			need: []string{"foo-1bar-2baz-3baz.txt"},
			args: args{
				filePath:          "foo-1bar-2baz-3baz.txt",
				regularExpression: "bar?z?",
				deleteText:        "-baz-baz",
				forceOverwrite:    false,
				dryRun:            true,
			},
			want: []string{"foo-1bar-2baz-3baz.txt"},
		},
		{
			name: "complex",
			need: []string{"foo-1080p-0pro-bar-2ffc.txt"},
			args: args{
				filePath: "foo-1080p-0pro-bar-2ffc.txt",
				// regularExpression: "halfpro|pro|amat",
				deleteText:     "-ffc",
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-1080p-2pro-bar.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)
			result := mergeParts(fi, tt.args.regularExpression, tt.args.deleteText, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_prefix(t *testing.T) {
	type args struct {
		filePath       string
		newPart        string
		skip           int
		forceOverwrite bool
		dryRun         bool
		verbose        bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"1.txt"},
			args: args{
				filePath:       "1.txt",
				newPart:        "prefix",
				skip:           0,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"prefix-1.txt"},
		},
		{
			name: "skip-one",
			need: []string{"foo-1.txt"},
			args: args{
				filePath:       "foo-1.txt",
				newPart:        "prefix",
				skip:           1,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-prefix-1.txt"},
		},
		{
			name: "skip-to-last",
			need: []string{"1.txt"},
			args: args{
				filePath:       "1.txt",
				newPart:        "prefix",
				skip:           1,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"1-prefix.txt"},
		},
		{
			name: "skip-to-last",
			need: []string{"1.txt", "1-prefix.txt"},
			args: args{
				filePath:       "1.txt",
				newPart:        "prefix",
				skip:           1,
				forceOverwrite: true,
				dryRun:         false,
			},
			want: []string{"1-prefix.txt"},
		},
		{
			name: "dry run",
			need: []string{"1.txt", "1-prefix.txt"},
			args: args{
				filePath:       "1.txt",
				newPart:        "prefix",
				skip:           1,
				forceOverwrite: true,
				dryRun:         true,
			},
			want: []string{"1.txt", "1-prefix.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)
			result := prefix(fi, tt.args.newPart, tt.args.skip, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_reEncode(t *testing.T) {
	type args struct {
		filePath string
		codec    string
		crf      int
		preset   string
		hwaccel  string
		dryRun   bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "libx264",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "libx264",
				crf:      51,
				preset:   "veryfast",
				dryRun:   false,
			},
			want: []string{"foo-libx264-51-veryfast.mp4"},
		},
		{
			name: "libx264 default crf",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "libx264",
				crf:      0,
				preset:   "veryfast",
				dryRun:   false,
			},
			want: []string{"foo-libx264-23-veryfast.mp4"},
		},
		{
			name: "libx265",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "libx265",
				crf:      25,
				preset:   "ultrafast",
				dryRun:   false,
			},
			want: []string{"foo-libx265-25-ultrafast.mp4"},
		},
		{
			name: "libx265 default crf",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "libx265",
				crf:      0,
				preset:   "ultrafast",
				dryRun:   false,
			},
			want: []string{"foo-libx265-28-ultrafast.mp4"},
		},
		{
			name: "vp9",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "vp9",
				crf:      63,
				preset:   "slow",
				dryRun:   false,
			},
			want: []string{"foo-vp9-63.mkv"},
		},
		{
			name: "vp9 default crf",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "vp9",
				crf:      0,
				dryRun:   false,
			},
			want: []string{"foo-vp9-lossless.mkv"},
		},
		{
			name: "vp9 default crf",
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				codec:    "vp9",
				crf:      0,
				dryRun:   false,
			},
			want: []string{"foo-vp9-lossless.mkv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				_ = os.Remove(filePath)
				createExampleVideo(t, filePath)
			}
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)

			// execute
			_, result := reEncode(fi, tt.args.codec, tt.args.crf, tt.args.preset, tt.args.hwaccel, "", tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_replace(t *testing.T) {
	type args struct {
		filePath       string
		search         string
		replaceWith    string
		skip           int
		forceOverwrite bool
		dryRun         bool
		verbose        bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo.txt"},
			args: args{
				filePath:       "foo.txt",
				search:         "foo",
				replaceWith:    "bar",
				skip:           0,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"bar.txt"},
		},
		{
			name: "replace first find",
			need: []string{"foo-foo.txt"},
			args: args{
				filePath:       "foo-foo.txt",
				search:         "foo",
				replaceWith:    "bar",
				skip:           0,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"bar-foo.txt"},
		},
		{
			name: "replace second find",
			need: []string{"foo-foo.txt"},
			args: args{
				filePath:       "foo-foo.txt",
				search:         "foo",
				replaceWith:    "bar",
				skip:           1,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-bar.txt"},
		},
		{
			name: "replace non-part find",
			need: []string{"foo-foo.txt"},
			args: args{
				filePath:       "foo-foo.txt",
				search:         "foo-f",
				replaceWith:    "bar",
				skip:           0,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"baroo.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)
			result := replace(fi, tt.args.search, tt.args.replaceWith, tt.args.skip, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_safeRename(t *testing.T) {
	type args struct {
		oldPath        string
		newPath        string
		forceOverwrite bool
	}
	tests := []struct {
		name    string
		need    []string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "rename",
			need: []string{"1.txt"},
			args: args{
				oldPath:        "1.txt",
				newPath:        "2.txt",
				forceOverwrite: false,
			},
			want:    []string{"2.txt"},
			wantErr: false,
		},
		{
			name: "file names match",
			need: []string{"1.txt"},
			args: args{
				oldPath:        "1.txt",
				newPath:        "1.txt",
				forceOverwrite: false,
			},
			want:    []string{"1.txt"},
			wantErr: false,
		},
		{
			name: "overwrite",
			need: []string{"1.txt", "2.txt"},
			args: args{
				oldPath:        "1.txt",
				newPath:        "2.txt",
				forceOverwrite: true,
			},
			want:    []string{"2.txt"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, fileName := range tt.need {
				err = os.WriteFile(fileName, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			if err := safeRename(tt.args.oldPath, tt.args.newPath, tt.args.forceOverwrite); (err != nil) != tt.wantErr {
				t.Errorf("safeRename() error = %v, wantErr %v", err, tt.wantErr)
			}

			// assert
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_suffix(t *testing.T) {
	type args struct {
		filePath       string
		newPart        string
		skip           int
		forceOverwrite bool
		dryRun         bool
		verbose        bool
	}
	tests := []struct {
		name string
		need []string
		args args
		want []string
	}{
		{
			name: "default",
			need: []string{"foo.txt"},
			args: args{
				filePath:       "foo.txt",
				newPart:        "BAR",
				skip:           0,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-BAR.txt"},
		},
		{
			name: "multiple",
			need: []string{"foo-bar.txt"},
			args: args{
				filePath:       "foo-bar.txt",
				newPart:        "BAZ",
				skip:           0,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-bar-BAZ.txt"},
		},
		{
			name: "multiple with skipping 1",
			need: []string{"foo-bar.txt"},
			args: args{
				filePath:       "foo-bar.txt",
				newPart:        "BAZ",
				skip:           1,
				forceOverwrite: false,
				dryRun:         false,
			},
			want: []string{"foo-BAZ-bar.txt"},
		},
		{
			name: "dry run",
			need: []string{"foo.txt"},
			args: args{
				filePath:       "foo.txt",
				newPart:        "BAR",
				skip:           0,
				forceOverwrite: false,
				dryRun:         true,
			},
			want: []string{"foo.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				err = os.WriteFile(filePath, nil, 0777)
				require.NoError(t, err)
			}

			// execute
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)
			result := suffix(fi, tt.args.newPart, tt.args.skip, tt.args.forceOverwrite, tt.args.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
		})
	}
}

func Test_crop(t *testing.T) {
	type args struct {
		filePath          string
		regularExpression string
		insertText        string
		forceOverwrite    bool
		dryRun            bool
		width, height     int
		x, y              string
		dimensionPreset   string
	}
	tests := []struct {
		name       string
		need       []string
		args       args
		wantOutput string
		want       []string
	}{
		{
			name: "default-120-80-left-top",
			// 320x240
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				dryRun:   false,
				width:    120,
				height:   80,
				x:        "left",
				y:        "top",
			},
			wantOutput: "ffmpeg -i \"foo.mp4\" -filter:v \"crop=120:80:0:0\" \"foo-120x80.mp4\"",
			want:       []string{"foo-120x80.mp4"},
		},
		{
			name: "default-120-80-center-center",
			// 320x240
			need: []string{"foo.mp4"},
			args: args{
				filePath: "foo.mp4",
				dryRun:   false,
				width:    120,
				height:   80,
				x:        "center",
				y:        "center",
			},
			wantOutput: "ffmpeg -i \"foo.mp4\" -filter:v \"crop=120:80:100:80\" \"foo-120x80.mp4\"",
			want:       []string{"foo-120x80.mp4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer cleanUp(t, tt.want, tt.need)

			var err error

			// setup
			for _, filePath := range tt.need {
				_ = os.Remove(filePath)
				createExampleVideo(t, filePath)
			}
			fi, err := os.Stat(tt.args.filePath)
			require.NoError(t, err)

			// execute
			a := tt.args
			result := crop(fi, a.width, a.height, a.x, a.y, a.dimensionPreset, a.forceOverwrite, a.dryRun)

			// assert
			assert.NoError(t, result)
			for _, fileName := range tt.want {
				assert.FileExists(t, fileName)
			}
			assert.Contains(t, l.history, tt.wantOutput)
		})
	}
}
