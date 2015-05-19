<center>
# lstoc.go
</center>

## OVERVIEW

lstoc will list the file names inside "collections" recursively. In other words, if
a tar file contains a zip file as one of its members lstoc will also list the
contents (again recursively) of the zip file.

The most familiar 
collections are probably tar or zip archives. The program will attempt to 
unpack/uncompress/untar/unzip/un-whatever whenever possible.

While this program does not operate on the contents of
these archives it provides a foundation for a program that could.

### Usage examples

* lstoc file.tgz
* lstoc *.tgz *.zip
* find /cdrom -name *.zip | lstoc

### Installation

If you have a working go installation on a Unix-like OS:

> ```go get github.com/hotei/lstoc```

Will copy github.com/hotei/lstoc to the first entry of your $GOPATH

or if go is not installed yet :

> ```cd DestinationDirectory```

> ```git clone https://github.com/hotei/lstoc.git```

### Features

* easy to extend to other compression metohods
* will go up to 15 levels deep (compile time limit that's modifiable)

### Limitations

* skips collection if larger than MaxFileSize (1 GB at present) Driven somewhat
by requirement that whole collection resides in memory while being worked on.
* not suitable for use with encrypted zip methods.  Will not attempt to decrypt
a central directory or individual files.


### Why

I have tens of thousands of archived files, some quite old.  I sometimes want to see if they
are still readable without having to untar/unpack/uncompress/unzip everything manually.
This program works for about 99.5% of my collections. It works with disk and flash and
optical media such as CD and DVD.

```
$ find /media/diskA -type f | lstoc
```

Sometimes I want to see if I have an old version of a file that might have been
archived years ago somewhere inside a 500 MB zip that represents an entire drive.
That's pretty simple now :

```
lstoc 500MB.zip | grep -i targetfile.c
```

Might even work with non-seekable media such as DAT with a few slight mods.  Since
we scan headers sequentially there's no real need for an io.ReadSeeker as source.
My zipfile package would be trivial to modify in this fashion.

### Scheduled To-Do
* BUGS
  * TBD
* Essential:
  * TBD
* Nice:
  * Implement zip implode/explode method (files present but rare)
  	* see partial progress in http://github.com/hotei/zipfile project
* Nice but no immediate need:
  * MaxFileSize settable with flag ?
  * Add more collections / compression methods
  
### Change Log

* 2015-05-12 update for go 1.4.2
* 2014-04-07 update for go 1.2.1
* 2013-03-25 
  * started implode translation which turns out to be quite non-trivial
  * moved archive pattern code to mdr since it's useful in other contexts
* 2013-03-09 update for go 1.0.3
* 2012-03-18 reorganized to use dispatcher so that arbitrary nesting now works
* 2012-03-17 Started

 
### Collections already done

List of collections/compressors recognized in alpha order

*	gz			- not a collection by itself, the gzip compression method	
*	tar			- "tape" archive - the commonest collection in Unix environments
*	tar.bz 		- bz2 compressed tar (almost always bz2 despite lack of 2, checks magic#)
*	tar.bz2 	- bz2 compressed tar
*	tar.gz 		- gzipped tar
*	tar.Z		- Z compressed tar
*	tbz 		- bz2 compressed tar (almost always bz2 despite lack of 2, checks magic#)
*	tbz2 		- bz2 compressed tar
*	tgz 		- gzipped tar
*	zip			- may be a true collection or have only one member, might be compressed or not
	* - __stored__ data where no compression is used
	* - __deflated__ data is handled using archive/flate package
	* - __imploded__ data is being worked on (non-trivial C -> go translation)
*	Z			- the Unix _compress_ method is not a collection by itself, but 
can be used to compress collections - mbx.Z for instance

***

#### Misc To-Do

Add to recognized patterns in MDR

* gtar
* gnutar
* ustar
* taz?

Other collections not currently recognized:

Priority - Low

*	7z			- p7zip - usually lzma (possible collection)
*	a			- Unix archive (object code usually) (collection)
*	ace
*	alz
*	ar          - http://en.wikipedia.org/wiki/Ar_(Unix)
*	arc			- http://en.wikipedia.org/wiki/ARC_(file_format)
*	arj
*	as			- AppleSingle
*	bin			- Apple?
*	bz			- Unix (the original deprecated one, not bz2)
*	cab			- MicroSoft OEM software bundler (collection)
*	cbr
*	cbt
*	cbz
*	cpt			- CompactPro
*	cpio		- Unix CopyInOut, can be an archive
*	cpz
*	dar
*	deb			- Debian Linux software delivery vehicle
*	dump		- Unix _dump_ command output (collection)
*	enc
*	exe			- Zip (and possibly other) self-extracting (possible collection)
*	hqx			- Apple BinHex
*	img			- whole "disk" image from floppy to multi-gig flashdrive (collection)
*	iso			- CD / DVD image (collection)
*	jar			- [java archive][4]  (collection) 
*	kgb
*	larc		- similar to lha but different author
*	lha			- has multiple siblings
*	lzh			- see lha
*	lzma		- p7zip default compression method
*	lzs			- see LArc
*	lzx			- see lha
*	macbin		- Apple
*	mbx			- common email mailboxname
*	mime		- ? used for collections outside email?
*	msi			- MicroSoft OEM software bundler
*	rar			- http://en.wikipedia.org/wiki/RAR_Archive
*	rNN
*	rpm			- Linux RedHat Package Manager
*	sea			- Apple?
*	shar 		- shell archive (collection)
*	sit			- Apple hqx cousin?
*	sitx		- Apple hqx cousin?
*	sit.N		- Apple hqx cousin?
*	uu			- early mail attachments (used with uucp)
*	uue			- early mail attachments (used with uucp)
*	xar
*	y
*	ync
*	z			- Unix pack/unpack (rare)  may be handled by early gzip, see BSD 4.4 source gzip/zcat
*	zNN


* How best to use lstoc.Paranoid? - if at all?
	* as flag? -Paranoid=bool would indicate we want to set/clear Paranoid mode in zipfile etc perhaps
	* -BestEffort flag means to try as hard as possible to keep processing in spite of errors
	
#### NOTES

* Would LZW from pkg/compress work on compress'd files? (limits in LZW docs imply not but...)
* Currently uses find to pick up arguments.  Probably better than path.Walk
* resources for further development
	-	/usr/share/misc/magic.mgc and the "/usr/bin/file" command source
	-	wiki "file signatures", "magic number programming" and the various archive names

gz and bz2 are single file compression methods, not archivers.  They might be
found appended to any of the pure archivers (.cpio.gz etc)  bz is deprecated and
generally indicates bz2 compression though it's not guaranteed.  bz lived long enough
to have been used some but patent disputes killed it. 

***

[JAR header][4]

***

### Microsoft CAB header	

	struct CFHEADER
	{
	  u1  signature[4]inet file signature */
	  u4  reserved1     /* reserved */
	  u4  cbCabinet    /* size of this cabinet file in bytes */
	  u4  reserved2     /* reserved */
	  u4  coffFiles/* offset of the first CFFILE entry */
	  u4  reserved3     /* reserved */
	  u1  versionMinor   /* cabinet file format version, minor */
	  u1  versionMajor   /* cabinet file format version, major */
	  u2  cFolders  /* number of CFFOLDER entries in this */
							/*    cabinet */
	  u2  cFiles      /* number of CFFILE entries in this cabinet */
	  u2  flags        /* cabinet file option indicators */
	  u2  setID        /* must be the same for all cabinets in a */
							/*    set */
	  u2  iCabinet;         /* number of this cabinet file in a set */
	  u2  cbCFHeader;       /* (optional) size of per-cabinet reserved */
							/*    area */
	  u1  cbCFFolder;       /* (optional) size of per-folder reserved */
							/*    area */
	  u1  cbCFData;         /* (optional) size of per-datablock reserved */
							/*    area */
	  u1  abReserve[];      /* (optional) per-cabinet reserved area */
	  u1  szCabinetPrev[];  /* (optional) name of previous cabinet file */
	  u1  szDiskPrev[];     /* (optional) name of previous disk */
	  u1  szCabinetNext[];  /* (optional) name of next cabinet file */
	  u1  szDiskNext[];     /* (optional) name of next disk */
	};
	
***

### Resources

* [go language reference] [1] 
* [go standard library package docs] [2]
* [Source for program] [3]

[1]: http://golang.org/ref/spec/ "go reference spec"
[2]: http://golang.org/pkg/ "go package docs"
[3]: http://github.com/hotei/lstoc "github.com/hotei/lstoc"
[4]: http://docs.oracle.com/javase/6/docs/technotes/guides/jar/jar.html "oracle java doc"
[5]: http://msdn.microsoft.com/en-us/library/bb267310.aspx "MSoft CAB header"

Comments can be sent to <hotei1352@gmail.com> or to user "hotei" at github.com.

License
-------
The 'lstoc' go package/program is distributed under the Simplified BSD License:

> Copyright (c) 2015 David Rook. All rights reserved.
> 
> Redistribution and use in source and binary forms, with or without modification, are
> permitted provided that the following conditions are met:
> 
>    1. Redistributions of source code must retain the above copyright notice, this list of
>       conditions and the following disclaimer.
> 
>    2. Redistributions in binary form must reproduce the above copyright notice, this list
>       of conditions and the following disclaimer in the documentation and/or other materials
>       provided with the distribution.
> 
> THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDER ``AS IS'' AND ANY EXPRESS OR IMPLIED
> WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
> FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> OR
> CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
> CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
> SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
> ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
> NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF
> ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

Documentation (c) 2015 David Rook 

// end of lstoc.md
