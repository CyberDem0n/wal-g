package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/katie31/wal-g"
	"os"
	"path/filepath"
	"time"
)

func main() {
	/**
	 *  Configure and start session with bucket, region, and path names. Checks that environment variables
	 *  are properly set. Layer: Server
	 */
	tu, pre := walg.Configure()

	fmt.Println("BUCKET:", *pre.Bucket)
	fmt.Println("PATH:", *pre.Server)

	/*** Grab arguments from command line ***/
	all := os.Args
	command := all[1]
	dirArc := all[2]

	var backupName string
	if len(all) == 4 {
		backupName = all[3]
	}

	/*** OPTION: BACKUP-FETCH ***/
	if command == "backup-fetch" {
		var keys []string
		var bk *walg.Backup
		/*** Check if backup specified in COMMAND LINE exists and if it does extract to NEWDIR. ***/
		if backupName != "LATEST" {
			bk = &walg.Backup{
				Prefix: pre,
				Path:   aws.String(*pre.Server + "/basebackups_005/"),
				Name:   aws.String(backupName),
			}

			bk.Js = aws.String(*bk.Path + *bk.Name + "_backup_stop_sentinel.json")

			fmt.Println("NEWDIR:", dirArc)
			fmt.Println("PATH:", *bk.Path)
			fmt.Println("NAME:", *bk.Name)
			fmt.Println("JSON:", *bk.Js)
			fmt.Println(bk.CheckExistence())

			if bk.CheckExistence() {
				keys = walg.GetKeys(bk)
			} else {
				fmt.Printf("Backup '%s' does not exist.\n", *bk.Name)
				os.Exit(1)
			}

			/*** Find the LATEST valid backup (checks against JSON file and grabs name from there) and extract to NEWDIR. ***/
		} else {
			bk = &walg.Backup{
				Prefix: pre,
				Path:   aws.String(*pre.Server + "/basebackups_005/"),
			}

			bk.Name = aws.String(walg.GetLatest(bk))
			keys = walg.GetKeys(bk)

			fmt.Println("NEWDIR", dirArc)
			fmt.Println("PATH:", *bk.Path)
			fmt.Println("NAME:", *bk.Name)

		}

		out := make([]walg.ReaderMaker, len(keys))
		for i, key := range keys {
			s := &walg.S3ReaderMaker{
				Backup:     bk,
				Key:        aws.String(key),
				FileFormat: walg.CheckType(key),
			}
			out[i] = s
		}

		f := &walg.FileTarInterpreter{
			NewDir: dirArc,
		}

		walg.ExtractAll(f, out)

		//np := &walg.NOPTarInterpreter{}
		//walg.ExtractAll(np, out)
	} else if command == "wal-fetch" {
		a := &walg.Archive{
			Prefix:  pre,
			Archive: aws.String(*pre.Server + "/wal_005/" + dirArc + ".lzo"),
		}

		if a.CheckExistence() {
			arch := walg.GetArchive(a)
			f, err := os.Create(backupName)
			if err != nil {
				panic(err)
			}

			walg.DecompressLzo(f, arch)
			f.Close()
		} else if a.Archive = aws.String(*pre.Server + "/wal_005/" + dirArc + ".lz4"); a.CheckExistence() {
			arch := walg.GetArchive(a)
			f, err := os.Create(backupName)
			if err != nil {
				panic(err)
			}

			walg.DecompressLz4(f, arch)
			f.Close()
		} else {
			fmt.Printf("Archive '%s' does not exist.\n", dirArc)
			os.Exit(1)
		}
		// fmt.Println(*a.Archive)
		// fmt.Println(a.CheckExistence())
		//a.CheckExistence()

	} else if command == "wal-push" {
		tu.UploadWal(dirArc)
		tu.Finish()
	} else if command == "backup-push" {
		bundle := &walg.Bundle{
			MinSize: int64(1000000000), //MINSIZE = 1GB
		}
		c, err := walg.Connect()
		if err != nil {
			panic(err)
		}
		lbl, sc := walg.QueryFile(c, time.Now().String())
		n := walg.FormatName(lbl)

		bundle.Tbm = &walg.S3TarBallMaker{
			BaseDir:  filepath.Base(dirArc),
			Trim:     dirArc,
			BkupName: n,
			Tu:       tu,
		}

		/*** UPLOAD label files. ***/
		bundle.NewTarBall()
		bundle.UploadLabelFiles(lbl, sc)

		/*** WALK the in directory and upload to S3. ***/
		bundle.NewTarBall()
		defer walg.TimeTrack(time.Now(), "MAIN")
		fmt.Println("Walking ...")
		err = filepath.Walk(dirArc, bundle.TarWalker)
		if err != nil {
			panic(err)
		}
		bundle.Tb.CloseTar()
		bundle.Tb.Finish()
	}

}
