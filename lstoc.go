// lstoc.go  (c) 2012-2014 David Rook

package main

// BUG(mdr): WARNING: skips collection if larger than MaxFileSize (1G at present)

import (
	// go 1.4.2 stdlib pkgs
	"archive/tar"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	// my local pkgs
	"github.com/hotei/dcompress"
	"github.com/hotei/mdr"
	"github.com/hotei/zipfile"
	//"github.com/hotei/arkfile"
)

const (
	MaxArkDepth = 15
	GigaByte    = int64(1024 * 1024 * 1024)
	MaxFileSize = GigaByte
)

var (
	g_version = "lstoc.go v 0.0.2 (c) 2012-2015 David Rook all rights reserved"

	flagCPU       int     // cpus requested on command line
	nCPU          int = 8 // BUG(mdr): magic number
	g_verboseFlag bool
	g_logFlag     bool // ?
	g_profileFlag bool // create and save profile info

	g_logfile   *os.File
	g_startTime time.Time
	g_hostName  string // grabbed from /etc/hostname
	g_homeDir   string // users home directory

	g_processedArgCt        = 0
	g_processedByteCt int64 = 0
	g_argCt                 = 0
	g_fileCt                = 0
	g_arkDepth              = 0 // any style ark can increase depth

	//	ErrOther      = errors.New("ListArchTOC: Other error :-( ")  unused at present
	ErrExpanding  = errors.New("lstoc: Cant expand array ")
	ErrInvalidSig = errors.New("lstoc: a local header has an invalid magic number ")
	ErrNoUnPack   = errors.New("lstoc: unpack not implemented yet ")
	ErrNoImplode  = errors.New("lstoc: zip-implode not implemented yet ")
	ErrShortRead  = errors.New("lstoc: short read ")
	ErrTooDeep    = errors.New(fmt.Sprintf("lstoc: over %d levels deep ", MaxArkDepth))

	WriteLogErrs bool
	Paranoid     bool
	logFileName  = "/home/mdr/gologs/golog.txt"
	version      = "0.0.1"
)

func init() {
	now := time.Now()
	t := now.Format(time.UnixDate)
	WriteLogErrs = true
	logErr(fmt.Sprintf("lstoc: version %s init at %s\n", version, t))

	zipfile.WriteLogErrs = true
	zipfile.Paranoid = false
	// zipfile.Verbose = true
	g_startTime = time.Now()
	var err error // required so g_hostName wont become local
	g_hostName, err = os.Hostname()
	if err != nil || (len(g_hostName) == 0) {
		g_hostName = "UnknownHostName"
	}
	g_homeDir = os.Getenv("HOME")
	if len(g_homeDir) <= 0 {
		fmt.Printf("Can't open logfile because $HOME not set in environment\n")
		os.Exit(1)
	}

	//	flag.BoolVar(&g_logFlag, "log", false, "append log output to ~/gologs/ListArchTOC.log")
	flag.BoolVar(&g_profileFlag, "p", false, "run profiler")
	flag.BoolVar(&g_verboseFlag, "v", true, "verbose messages")
	flag.IntVar(&flagCPU, "cpu", 0, "Number of CPU cores to use(default is all available)")
}

func dispatch(fname string) func([]byte, string) error {
	archType := mdr.WhichArchiveType(fname)
	compType := mdr.WhichCompressType(fname)

	// one ext implies combined type like tgz or taz
	if (archType == mdr.ArchiveTarType) && (compType == mdr.CompressZcompressType) {
		return tZHeaderList
	}
	if (archType == mdr.ArchiveTarType) && (compType == mdr.CompressGzipType) {
		return tgzHeaderList
	}
	if (archType == mdr.ArchiveTarType) && (compType == mdr.CompressBz2Type) {
		return tbz2HeaderList
	}
	//	if (archType == mdr.ArchiveArkType) && (compType == mdr.CompressZcompressType) {
	//		return zarkHeaderList
	//	}
	if archType == mdr.ArchiveZipType {
		return zipHeaderList
	}

	// simple archives
	if (archType == mdr.ArchiveTarType) && (compType == mdr.CompressNoMatchType) {
		return tarHeaderList
	}
	//	if (archType == mdr.ArchiveArkType) && (compType == mdr.CompressNoMatchType) {
	//		return arkHeaderList
	//	}

	// if we get here then nothing matched
	return todoHeaderList
}

/*
func usage() {
	docString := `
usage: lstoc [-p] [-v] collectionName
		-p      save profile data
		-v      be verbose`
	fmt.Printf("%s\n", docString)
	os.Exit(0)
}
*/

// reads file list from Stdin
func getAllArgs() []string {
	rv := make([]string, 0, 100)
	if flag.NArg() == 0 {
		f := os.Stdin // f is * osFile
		rdr := bufio.NewReader(f)
		ct := 0
		for {
			line, err := rdr.ReadString('\n')
			ct++
			line = strings.TrimRight(line, "\n \r\t")
			if err != nil {
				if err == io.EOF { // need not end with nl
					if len(line) > 0 {
						rv = append(rv, line)
						g_argCt++
					}
					break
				}
			}
			Verbose.Printf("%6d %s\n", ct, line)
			rv = append(rv, line)
			g_argCt++
		}
		return rv
	}
	args := flag.Args()
	for _, arg := range args {
		rv = append(rv, arg)
		g_argCt++
	}
	return rv
}

func todoHeaderList(allBytes []byte, curPath string) error {
	Verbose.Printf("!Warning-> this file type NOT handled yet: %s\n", curPath)
	return nil
}

// Either taz or tar.Z compressed or tar.z or taz packed tar archive (yes it's ambiguous)
// must check magic number to be sure what we've got
func tZHeaderList(tZBytes []byte, curPath string) error {
	Verbose.Printf("tZHeaderList %s\n", curPath)
	//Verbose.Printf("%s could MATCH either -> tar.Z(z)\n", curPath)
	// doesn't matter if filename is .Z or .z, could be compress or pack.  Check magic
	if len(tZBytes) < 2 {
		return ErrShortRead
	}
	var buf = []byte{}
	Verbose.Printf("len(tZByes)= %d\n", len(tZBytes))
	if (tZBytes[0] == dcompress.MagicBytes[0]) &&
		(tZBytes[1] == dcompress.MagicBytes[1]) {
		// compress magic bytess matched
		Verbose.Printf("%s MATCHED -> magic says compressed tar.Z\n", curPath)
		if curPath[len(curPath)-1] == 'z' {
			fmt.Printf("!Warning-> extension and magic don't match for %s\n", curPath)
		}
		g_arkDepth++
		if g_arkDepth > MaxArkDepth {
			fmt.Printf("!Err-> over %d levels deep in arks? currently on %s\n", MaxArkDepth, curPath)
			g_arkDepth--
			return ErrTooDeep
		}
		// decompress, then pass to tarHeaderList for expansion of individual files
		r := bytes.NewReader(tZBytes)
		tzReader, err := dcompress.NewReader(r)
		if err != nil {
			fmt.Printf("!Err-> %v on %s\n", err, curPath)
			g_arkDepth--
			return err
		}
		buf, err = ioutil.ReadAll(tzReader)
		if err != nil {
			fmt.Printf("!Err-> %v on %s\n", err, curPath)
			g_arkDepth--
			return err
		}
		Verbose.Printf("read n2(%d) bytes from compressed-tar.Z\n", len(buf))
		tarHeaderList(buf, curPath)
		g_arkDepth--
		return nil
	}
	// was packed not compressed, so report failure, don't change arkdepth
	Verbose.Printf("read n2(%d) bytes from packed-tar.z\n", len(buf))
	return ErrNoUnPack
}

/*
func zarkHeaderList(arkZBytes []byte, curPath string) error {
	//Verbose.Printf("!Warning-> this file type NOT handled yet: %s\n", curPath)
	//return nil
	// ok to here fmt.Printf("len arkZBytes= %d\n",len(arkZBytes))
	Verbose.Printf("zarkHeaderList %s\n", curPath)
	//Verbose.Printf("%s could MATCH either -> tar.Z(z)\n", curPath)
	// doesn't matter if filename is .Z or .z, could be compress or pack.  Check magic
	if len(arkZBytes) < 2 {
		return ErrShortRead
	}
	Verbose.Printf("len(arkZByes)= %d\n", len(arkZBytes))
	var buf = []byte{}
	if (arkZBytes[0] == dcompress.MagicBytes[0]) &&
		(arkZBytes[1] == dcompress.MagicBytes[1]) {
		// compress magic bytess matched
		Verbose.Printf("%s MATCHED -> magic says compressed .Z\n", curPath)
		if curPath[len(curPath)-1] == 'z' {
			fmt.Printf("!Warning-> extension and magic don't match for %s\n", curPath)
		}
		g_arkDepth++
		if g_arkDepth > MaxArkDepth {
			fmt.Printf("!Err-> over %d levels deep in arks? currently on %s\n", MaxArkDepth, curPath)
			g_arkDepth--
			return ErrTooDeep
		}
		// decompress, then pass to arkHeaderList for expansion of individual files

		r := bytes.NewReader(arkZBytes)
		// dcompress requires an io.ReadSeeker (r) to create the Zreader
		arkzReader, err := dcompress.NewReader(r)
		if err != nil {
			fmt.Printf("!Err-> %v on %s\n", err, curPath)
			g_arkDepth--
			return err
		}
		buf, err = arkzReader.ReadAll()
		if err != nil {
			fmt.Printf("!Err-> %v on %s\n", err, curPath)
			g_arkDepth--
			return err
		}
		Verbose.Printf("read n2(%d) bytes from compressed-ark.Z\n", len(buf))
		arkHeaderList(buf, curPath)
		g_arkDepth--
		return nil
	}
	// was packed, not compressed, so report failure, don't change arkdepth
	Verbose.Printf("read n2(%d) bytes from packed-tar.z\n", len(buf))
	return ErrNoUnPack
}

func arkHeaderList(sharBytes []byte, curPath string) error {
	Verbose.Printf("arkHeaderList %s\n", curPath)
	g_arkDepth++
	if g_arkDepth > MaxArkDepth {
		fmt.Printf("!Err-> over %d levels deep in arks?", MaxArkDepth)
		g_arkDepth--
		return ErrTooDeep
	}
	r := bytes.NewReader(sharBytes)
	rs, err := arkfile.NewReader(r)
	if err != nil {
		g_arkDepth--
		return err
	}
	ndx := 0
	for {
		hdr, err := rs.Next()
		ndx++
		if err != nil {
			break
		}
		if len(hdr.Name) <= 0 {
			continue
		}
		fmt.Printf("%s//://%s\n", curPath, hdr.Name)
		g_fileCt++
	}
	g_arkDepth--
	return nil
}
*/

// decompress bzip2 before handoff to tar
// does not increase ark_Depth
func tbz2HeaderList(tbz2Bytes []byte, curPath string) error { // decompress bzip2 before handoff to tar
	Verbose.Printf("tbz2HeaderList %s\n", curPath)
	g_fileCt++
	// check magic number
	magicString := string(tbz2Bytes[0:3])
	if magicString != "BZh" {
		fmt.Printf("!Err-> bzip2 magic number(%q) should be (BZh) for %s\n", magicString, curPath)
		return ErrInvalidSig
	}
	// decompress, then pass to tarHeaderList for expansion
	// attach a bzip2 reader to the []byte
	r := bytes.NewReader(tbz2Bytes) // r is a slice reader
	bz2Reader := bzip2.NewReader(r) // not same interface as gzip.NewReader()
	/*
		if err != nil {
			fmt.Printf("!Err-> bzip2 read failed (%v) on %s\n", err, curPath)
			return err
		}
	*/
	b := new(bytes.Buffer)
	n2, err := io.Copy(b, bz2Reader)
	if err != nil {
		fmt.Printf("!Err-> bzip2 read failed (%v) on %s\n", err, curPath)
		return err
	}
	Verbose.Printf("read n2(%d) bytes from bzip2\n", n2)
	buf := b.Bytes()
	tarHeaderList(buf, curPath)
	return nil
}

// decompress gzip before handoff to tar
// does not increase ark_Depth
func tgzHeaderList(tgzBytes []byte, curPath string) error {
	Verbose.Printf("tgzHeaderList %s\n", curPath)
	g_fileCt++
	// decompress, then pass to tarHeaderList for expansion
	// attach a gzip reader to the []byte
	r := bytes.NewReader(tgzBytes) // r is a slice reader
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		fmt.Printf("!Err-> gzip read failed (%v) on %s\n", err, curPath)
		return err
	}
	defer gzReader.Close()
	b := new(bytes.Buffer)
	n2, err := io.Copy(b, gzReader)
	if err != nil {
		fmt.Printf("!Err-> gzip read failed (%v) on %s\n", err, curPath)
		return err
	}
	Verbose.Printf("read n2(%d) bytes from gzip\n", n2)
	buf := b.Bytes()
	tarHeaderList(buf, curPath)
	return nil
}

func tarHeaderList(tarBytes []byte, curPath string) error {
	g_arkDepth++
	if g_arkDepth > MaxArkDepth {
		fmt.Printf("!Err-> over %d levels deep in arks?", MaxArkDepth)
		g_arkDepth--
		return ErrTooDeep
	}
	// attach a reader to byte array
	r := bytes.NewReader(tarBytes) // * Reader
	rt := tar.NewReader(r)
	// read tarfile headers
	ndx := 0
	for {
		hdr, err := rt.Next()
		ndx++
		if err != nil {
			break // what does last file return ? TODO
		}
		if len(hdr.Name) <= 0 {
			continue
		}
		fmt.Printf("%s//://%s\n", curPath, hdr.Name)
		g_fileCt++
		archType := mdr.WhichArchiveType(hdr.Name)
		if archType != mdr.ArchiveNoMatchType {
			Verbose.Printf("collection %s found inside %s at hdr#(%d)\n",
				hdr.Name, curPath, ndx)
			// extract the blob from header and recurse downward
			blobBytes := make([]byte, 0, hdr.Size) // blob is uncompressed size
			// copy tar piece into blob
			blobBufr := bytes.NewBuffer(blobBytes)
			nw, err := io.Copy(blobBufr, rt)
			if err != nil {
				fmt.Printf("!Err-> copy of Blob failed %v\n", err)
				continue
			}
			if int64(nw) != hdr.Size {
				fmt.Printf("!Err-> Mismatch expected(%d) , Read(%d)\n", hdr.Size, nw)
				continue
			}
			Verbose.Printf("nw(%d), err(%v)\n", nw, err)
			// continue  		//========== DEBUG ONLY ===========
			newBuf := blobBufr.Bytes()
			newPath := curPath + "//://" + hdr.Name
			fun := dispatch(hdr.Name)
			fun(newBuf, newPath)
		}
	}
	g_arkDepth--
	return nil
}

func zipHeaderList(zipBytes []byte, curPath string) error {
	g_arkDepth++
	if g_arkDepth > MaxArkDepth {
		fmt.Printf("!Err-> over %d levels deep in arks", MaxArkDepth)
		g_arkDepth--
		return ErrTooDeep
	}
	// attach a reader to byte array
	r := bytes.NewReader(zipBytes) //  slice reader
	rz, err := zipfile.NewReader(r)
	if err != nil {
		fmt.Printf("!Err-> %v on %s\n", err, curPath)
		g_arkDepth--
		return err
	}
	// read zipfile headers
	hdrlist, err := rz.ZipfileHeaders()
	if err != nil {
		fmt.Printf("!Err-> Read headers failed (%v) on %s\n", err, curPath)
		g_arkDepth--
		return err
	}
	for ndx, hdr := range hdrlist {
		if len(hdr.FileName) <= 0 {
			continue
		}
		fmt.Printf("%s//://%s\n", curPath, hdr.FileName)
		g_fileCt++
		archType := mdr.WhichArchiveType(hdr.FileName)
		if archType != mdr.ArchiveNoMatchType {
			Verbose.Printf("collection %s found inside %s at hdr#(%d)\n",
				hdr.FileName, curPath, ndx)
			// extract the blob from header and recurse downward
			blobBytes := make([]byte, 0, hdr.Size) // blob is uncompressed size
			r, err := hdr.Open()
			if err != nil {
				hdr.Dump()
				fmt.Printf("!Err-> hdr open fail err %v\n", err)
				continue
			}
			// defer hdr.Close()  can't - has no close method
			blobBufr := bytes.NewBuffer(blobBytes)
			// copy the uncompressed stuff out
			nw, err := io.Copy(blobBufr, r)
			if err != nil {
				fmt.Printf("!Err-> ListArchTOC: copy fail err %v\n", err)
				continue
			}
			Verbose.Printf("nw(%d) err(%v)\n", nw, err)
			if nw <= 0 {
				// probably just read a directory, how to tell? last char is /
			}
			newBuf := blobBufr.Bytes()
			newPath := curPath + "//://" + hdr.FileName
			fun := dispatch(hdr.FileName)
			fun(newBuf, newPath)
		}
	}
	g_arkDepth--
	return nil
}

func flagSetup() {
	flag.Parse()
	// do this before checking profile or CPU stuff
	if g_logFlag {
		var err error
		log.SetFlags(log.Lshortfile)

		logName := g_homeDir + "/gologs/ListArchTOC.log"
		fmt.Printf("\nlogName(%s)\n", logName)
		g_logfile, err = os.OpenFile(logName, os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Panicf("Couldn't open %s (%s)", logName, err)
		}
		g_logfile.Seek(0, 2) // seek to end of file first, maybe not needed but ...
		Verbose.Printf("\n\n<ListArchTOC on %s>\n", g_hostName)
		msg := fmt.Sprintf("Logfile %s opened on %s at %s\n",
			logName, g_hostName, g_startTime.String())
		_, err = g_logfile.Write([]byte(msg))
	}

	// -cpu=n
	var NUM_CORES int = runtime.NumCPU()
	Verbose.Printf("CPUs from command line = %d\n", flagCPU)
	Verbose.Printf("NumCPU(%d)\n", NUM_CORES)
	Verbose.Printf("GOMAXPROCS(%q)\n", os.Getenv("GOMAXPROCS"))
	if flagCPU != 0 { // it was set, so force to reasonable value
		nCPU = flagCPU
		if flagCPU >= NUM_CORES {
			nCPU = NUM_CORES
		}
		if flagCPU < 0 {
			nCPU = 1
		}
	} else { // default to MAX
		nCPU = NUM_CORES
	}
	Verbose.Printf("setting GOMAXPROCS to %d (nCPU)\n", nCPU)
	runtime.GOMAXPROCS(nCPU)

	// -verbose flag
	dcompress.VerboseFlag = g_verboseFlag
	//arkfile.VerboseFlag = g_verboseFlag
	zipfile.Verbose = zipfile.VerboseType(g_verboseFlag)

}

// if arg is a collection dispatch it to the appropriate inspector func
// otherwise just print the name and return
func arkInspector(fname string) {
	if mdr.WhichArchiveType(fname) == mdr.ArchiveNoMatchType {
		Verbose.Printf("arkInspector() %s\n", fname)
		return
	}
	Verbose.Printf("ListArchTOC: inspecting collection %s\n", fname)
	fi, err := os.Stat(fname)
	if err != nil {
		fmt.Printf("!Err-> Stat failed on %s err %v\n", fname, err)
	}
	FileBytes, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Printf("!Err-> Can't read %s\n", fname)
		return
	}
	Verbose.Printf("ListArchTOC: Read collection %s ok\n", fname)
	g_processedByteCt += fi.Size()
	g_processedArgCt++
	fun := dispatch(fname)
	fun(FileBytes, fname)
}

func main() {
	if g_verboseFlag {
		Verbose = true
	}
	flagSetup()
	Verbose = VerboseType(g_verboseFlag)
	Verbose.Printf("%s\n", g_version)
	if g_profileFlag { // NOTE BENE: this must stay in main() for defer to work properly
		proflogName := "ListArchTOC.prof"
		proflog, err := os.Create(proflogName)
		if err != nil {
			fmt.Printf("quitting because %v", err)
			os.Exit(-1)
		}
		pprof.StartCPUProfile(proflog)
		defer pprof.StopCPUProfile()
		Verbose.Printf("g_profileFlag true, profiling data in %s\n", proflogName)
	}

	Verbose.Printf("Before getAllArgs, ArgList = %v\n", os.Args)
	if len(os.Args) < 2 {
		fmt.Printf("improper usage, nothing to do\n")
		os.Exit(1)
	}
	argList := getAllArgs()
	Verbose.Printf("After getAllArgs, ArgList = %v\n", argList)
	if len(argList) <= 0 {
		fmt.Printf("improper usage, nothing to do\n")
		os.Exit(1)
	}

	for _, arg := range argList {
		fi, err := os.Stat(arg)
		if err != nil {
			fmt.Printf("!Err-> ListArchTOC: arg %s err %v\n", arg, err)
			continue
		}
		// check for normal readable file (not pipe or link etc)
		reg, err := mdr.FileIsRegular(arg)
		if err != nil {
			fmt.Printf("!Err-> ListArchTOC: arg %s err %v\n", arg, err)
			continue
		}
		if !reg {
			fmt.Printf("!Err-> arg %s not regular file\n", arg)
			continue
		}
		// check for local file ie no .gvfs in name
		_, filepart := path.Split(arg)
		if strings.Contains(filepart, ".gvfs") {
			fmt.Printf("# Skipping external file %s\n", arg)
			continue
		}
		if fi.IsDir() {
			// nothing to do, not even print
			continue
		}
		if fi.Size() > MaxFileSize {
			fmt.Printf("!Warning-> ListArchTOC: skipping %s, size(%d) is bigger than MAX allowed(%d)\n", arg, fi.Size(), MaxFileSize)
			continue
		}
		g_arkDepth = 0
		arkInspector(arg)
	}

	elapsed := time.Since(g_startTime)
	Verbose.Printf("Any errors with bad Date/Time stamps were ignored, use checkPath to detect those if needed\n")
	Verbose.Printf("items inspected(%d) collections found(%d)\n", g_argCt, g_processedArgCt)
	Verbose.Printf("Total file count(%d)\n", g_fileCt)
	Verbose.Printf("Raw collection size (may contain compressed files) = %.3g\n", float32(g_processedByteCt))
	Verbose.Printf("Processing time was %s\n", mdr.HumanTime(elapsed))
	Verbose.Printf("ListArchTOC on %s\n", g_hostName)

}

func logErr(s string) {
	if len(s) <= 0 {
		return
	}
	s = strings.TrimRight(s, "\r\n\t ") + "\n"
	fmt.Printf("%s", s)
	if !WriteLogErrs {
		return
	}
	//Verbose.Printf("opening logfile %s\n", logFileName)
	//f, err := os.OpenFile(logFileName,os.O_APPEND, 0640)
	// ? fails if file empty when first called, must use RDWR initially?
	f, err := os.OpenFile(logFileName, os.O_RDWR, 0640)
	if err != nil {
		Verbose.Printf("no existing log - opening new logfile %s\n", logFileName)
		f, err = os.Create(logFileName)
		if err != nil {
			log.Panicf("cant create log file\n")
		}
		defer f.Close()
	} else {
		//Verbose.Printf("opened %s\n", logFileName)
		defer f.Close()
	}
	//Verbose.Printf("orig name of file = %s\n", f.Name())
	n, err := f.Seek(0, 2)
	if err != nil {
		log.Panicf(fmt.Sprintf("cant seek end of log file:%s\n", err))
	}
	if false {
		Verbose.Printf("seek returned starting point of %d\n", n)
	}
	_, err = f.WriteString(s)
	if err != nil {
		log.Panicf(fmt.Sprintf("cant extend log file:%s\n", err))
	}
}

// save list of collections only (argList)
// somewhat useful if you want to compare the list we should be processing to what
// was acutally done.  Just clutter after that.
func deadCode_saveArgList(args []string) {
	fp, err := os.Create("collections.lst")
	if err != nil {
		fmt.Printf("Cant create %s\n", "collections.lst")
		os.Exit(-1)
	}
	for _, s := range args {
		s = fmt.Sprintf("%s\n", s)
		nw, err := fp.WriteString(s)
		_ = nw
		if err != nil {
			fmt.Printf("Cant create %s\n", "collections.lst")
			os.Exit(-1)
		}
	}
	fp.Close()
}

// probably could be simpler, see variadic version of verbose
func deadCode_listLog(s string) {
	s = strings.TrimRight(s, "\n \r\t")
	fmt.Printf("%s\n", s)
	if g_logFlag == true {
		b := fmt.Sprintf("%s\n", s)
		_, err := g_logfile.Write([]byte(b))
		if err != nil {
			log.Panicf("Couldn't write to logfile (%s)", err)
		}
	}
}

// adds PID to profile name so we get a different one every time
func deadCode_AddPID() {
	/*
		if false {
			log_procname += strconv.Itoa(os.Getpid())
		}
		log_procname += ".log"
			var tmpf *os.File
			tmpf, err = os.Open(log_procname)
			if err == nil {
				tmpf.Close()
				err = os.Remove(logname)
				if err != nil {
					log.Panicf("Couldn't remove %s at start of run (%s)", logname, err)
				}
			}
	*/
}
