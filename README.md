# vgrep

**vgrep** is a Python script to search strings in a given source tree.  It is
inspired by **cgvg**, but faster by using 'git grep'.

The script is in an early version.  Feedback & patches are always welcome.  Feel
free to copy, change, distribute and share.

##Usage Example:

###Searching a symbol
You can search for all occurrences of a specified string 'FOO' in your current
directory by calling **vgrep FOO**.  You can also specify multiple arguments
to search for.  vgrep prints the occurrences in the format "Index  Source File
Source Line  Content".  The index can later be used to open a specific location
in an editor.  vgrep caches the results of the last call, so that you can
re-open your previous results by simply calling **vgrep** without arguments.

![](https://github.com/vrothberg/vgrep/blob/master/screenshots/grep_example.png)

Note that vgrep pipes the output to 'less' for outputs bigger than 100 indexes.
You can turn this behavior off with **'--no-less'** so that the entire output
will be printed on the console.

If you're outside a Git repository or just don't like 'git grep' pass
**'--no-git'** to use to 'grep' instead.

###Show indexed location
To visit a specific location pass **'--show INDEX'**.  vgrep will then open the
location pointed to by *INDEX* with the editor that is set in your *enviroment*.
vgrep defaults to *vim* if the *EDITOR* environment variable is not set.

```
[~/linux/kernel/irq]$ export EDITOR=gedit
[~/linux/kernel/irq]$ vgrep --show 2
```

![](https://github.com/vrothberg/vgrep/blob/master/screenshots/show_example.png)
