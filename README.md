# vgrep

**vgrep** is a Python script to grep for strings in a given source tree.  It is
inspired by **cgvg**, but faster by making use of 'git grep'.

##Usage Example:

###Grep for a symbol
You can grep for all occurences of a specified string 'FOO' in your current
directory by calling **vgrep FOO**.  You can also specify multiple arguements
to search for.  vgrep prints the occurences in the format "Index  Source File
Source Line  Content".  The index can later be used to open a specific location
in an editor.  vgrep caches the results of the last call, so that you can
re-open your previous call by simply calling **vgrep** without arguments.

Note that vgrep pipes the output to 'less' for more than 100 indexes.  You can
turn this behavior off with **'--no-less'** so that the entire output will be
printed to the console.

If you're outside a Git repository or just don't like 'git grep' pass
**'--no-git'** to use to 'grep' instead.

```
[~/linux/kernel/irq]$ vgrep request_irq
Index  Source File  Source Line  Content

0      devres.c     129          *  free IRQs allocated with devm_request_irq().
1      manage.c     1391         *  free_irq - free an interrupt allocated with request_irq
2      manage.c     1572         ret = request_irq(irq, handler, flags, name, dev_id);
3      manage.c     483          KERN_ERR "enable_irq before setup/request_irq: irq %u\n", irq))
4      manage.c     559          int can_request_irq(unsigned int irq, unsigned long irqflags)
```

###Show indexed location
To visit a specific location we can use **'--show'** and the corresponding
index.  vgrep will then open the location with the editor set in your
*enviroment*.  If no editor is set, vgrep defaults to vim.

```
[~/linux/kernel/irq]$ vgrep --show 3
```
