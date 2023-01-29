# ffr

ffr is a toolbox which helps with cleaning up after extensive [ffc](https://github.com/peteraba/ffc) usage.

## Commands

### Re-encode

#### Simple

Re-encode `myvid.mpg` the default codec (`libx264`).

```
ffr r myvid.mpg
```

This will result in a new file called `myvid.mpg.mp4`.

#### Using vp9

Re-encode `myvid.mpg` using `vp9`.

```
ffr r --codec=vp9 myvid.mpg
```

This will result in a new file called `myvid.mpg.mkv`.

#### Using vp9 providing crf

About [CRF]([https://slhck.info/video/2017/02/24/crf-guide.html]).

Re-encode `myvid.mpg` using `vp9`, more compressed than before.

```
ffr r --codec=vp9 --crf=28 myvid.mpg
```

This will result in a new file called `myvid.mpg.mkv`.

#### Re-encoding multiple files at once

About [CRF]([https://slhck.info/video/2017/02/24/crf-guide.html]).

Re-encode `myvid.mpg` and `myvid2.mpg` using `vp9`.

```
ffr reencode --codec=vp9 --crf=28 myvid.mpg myvid2.mpg
```

This will result in two new files called `myvid.mpg.mkv` and `myvid2.mpg.mpg`.

### Append (suffix)

#### Simple

Append the same text to the end of a few files.

```
ffr a 'paris-france' 1.mpg 2.wmv 3.jpg 4.mp4
```

This will result in renaming the given files to the following:

1. 1paris-france.mpg
2. 2paris-france.wmv
3. 3paris-france.jpg
4. 4paris-france.mp4

#### Adding a dash

Append the same text to the end of a few files.

```
ffr a -s 'paris-france' 1.mpg 2.wmv 3.jpg 4.mp4
```

This will result in renaming the given files to the following:

1. 1-paris-france.mpg
2. 2-paris-france.wmv
3. 3-paris-france.jpg
4. 4-paris-france.mp4

### Prepend (prefix)

#### Simple

Prepend the same text to the beginning of a few files.

```
ffr p 'paris-france' 1.mpg 2.wmv 3.jpg 4.mp4
```

This will result in renaming the given files to the following:

1. paris-france1.mpg
2. paris-france2.wmv
3. paris-france3.jpg
4. paris-france4.mp4

#### Adding a dash

Append the same text to the end of a few files.

```
ffr a -s 'paris-france' 1.mpg 2.wmv 3.jpg 4.mp4
```

This will result in renaming the given files to the following:

1. paris-france-1.mpg
2. paris-france-2.wmv
3. paris-france-3.jpg
4. paris-france-4.mp4

### Merge

#### Simple

If you use [ffc](https://github.com/peteraba/ffc) multiple times on the same file, you'll have a weird filenames like `myvid-13ffc-1cut.mp4`. Merge helps with that, allowing you to clean up your files fast.

```
ffr m myvid-13ffc-1ffc.mp4
```

This will result in renaming `myvid-13ffc-1ffc.mp4` to `myvid-13cut.mp4`. This is because the number often comes from slicing a larger video into smaller ones, whereas the last description is often more descriptive of the particular file.

#### Keeping a different description

If you use [ffc](https://github.com/peteraba/ffc) multiple times on the same file, you'll have a weird filenames like `myvid-13ffc-1cut.mp4`. Merge helps with that, allowing you to clean up your files fast.

```
ffr m --keep=2 myvid-13ffc-1pele-shooting-from-50meters-1ffc.mp4
```

This will result in renaming `myvid-13ffc-1pele-shooting-from-50meters-1ffc.mp4` to `myvid-13pele-shooting-from-50meters.mp4`. This is because the number often comes from slicing a larger video into smaller ones, whereas the last description is often more descriptive of the particular file.

### Replace

Replace a text in the provided files.

```
ffr replace 'shooting' 'scoring'
```

This will result in renaming `myvid-13ffc-1pele-shooting-from-50meters-1ffc.mp4` to `myvid-13ffc-1pele-scoring-from-50meters-1ffc.mp4`.
