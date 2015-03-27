# vgrep

**vgrep** is a Python script to search strings in a given source tree.  It is
inspired by **cgvg**, but faster by making use of 'git grep'.

The script is in an early version.  Feedback & patches are always welcome.  Feel
free to copy, change, distribute and share.

##Usage Example:

###Grep for a symbol
You can grep for all occurrences of a specified string 'FOO' in your current
directory by calling **vgrep FOO**.  You can also specify multiple arguments
to search for.  vgrep prints the occurrences in the format "Index  Source File
Source Line  Content".  The index can later be used to open a specific location
in an editor.  vgrep caches the results of the last call, so that you can
re-open your previous call by simply calling **vgrep** without arguments.

Note that vgrep pipes the output to 'less' for more than 100 indexes.  You can
turn this behavior off with **'--no-less'** so that the entire output will be
printed to the console.

If you're outside a Git repository or just don't like 'git grep' pass
**'--no-git'** to use to 'grep' instead.

![](https://github.com/vrothberg/vgrep/blob/master/screenshots/grep_example.png)

###Show indexed location
To visit a specific location we can use **'--show'** and the corresponding
index.  vgrep will then open the location with the editor set in your
*enviroment*.  vgrep defaults to vim if the *EDITOR* environment variable is not
set.

```
[~/linux/kernel/irq]$ export EDITOR=gedit
[~/linux/kernel/irq]$ vgrep --show 2
```

![](https://github.com/vrothberg/vgrep/blob/master/screenshots/show_example.png)
