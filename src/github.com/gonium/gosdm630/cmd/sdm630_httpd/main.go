package main

import (
	"github.com/gonium/gosdm630"
	"gopkg.in/urfave/cli.v1"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	app := cli.NewApp()
	app.Name = "sdm630_httpd"
	app.Usage = "SDM630 power measurements via HTTP."
	app.Version = sdm630.RELEASEVERSION
	app.HideVersion = true
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "serialadapter, s",
			Value: "/dev/ttyUSB0",
			Usage: "path to serial RTU device",
		},
		cli.IntFlag{
			Name:  "comset, c",
			Value: sdm630.ModbusComset9600,
			Usage: `which communication parameter set to use. Valid sets are
		` + strconv.Itoa(sdm630.ModbusComset2400) + `:  2400 baud, 8N1
		` + strconv.Itoa(sdm630.ModbusComset9600) + `:  9600 baud, 8N1
		` + strconv.Itoa(sdm630.ModbusComset19200) + `: 19200 baud, 8N1
			`,
		},
		cli.StringFlag{
			Name:  "url, u",
			Value: ":8080",
			Usage: "the URL the server should respond on",
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "print verbose messages",
		},
		cli.StringFlag{
			Name:  "device_list, d",
			Value: "1",
			Usage: `MODBUS device type and ID to query, separated by comma.
			Valid types are:
			"SDM" for Eastron SDM meters
			"JANITZA" for Janitza B-Series DIN-Rail meters
			Example: -d JANITZA:1,SDM:22,SDM:23`,
		},
		cli.StringFlag{
			Name:  "unique_id_format, f",
			Value: "Instrument%d",
			Usage: `Unique ID format.
			Example: -f Instrument%d
			The %d is replaced by the device ID`,
		},
	}
	app.Action = func(c *cli.Context) {
		status := sdm630.NewStatus()

		// Set unique ID format
		sdm630.UniqueIdFormat = c.String("unique_id_format")

		// Parse the device_list parameter
		//deviceslice := strings.Split(c.String("device_list"), ",")
		//devids := make([]uint8, 0, len(deviceslice))
		//for _, devid := range deviceslice {
		//	id, err := strconv.Atoi(devid)
		//	if err != nil {
		//		log.Fatalf("Error parsing device id %s: %s", devid, err.Error())
		//	}
		//	devids = append(devids, uint8(id))
		//}

		// Parse the device_list parameter
		deviceslice := strings.Split(c.String("device_list"), ",")
		meters := make([]*sdm630.Meter, 0, len(deviceslice))
		for _, meterdef := range deviceslice {
			var meter *sdm630.Meter
			splitdef := strings.Split(meterdef, ":")
			if len(splitdef) != 2 {
				log.Fatalf("Cannot parse device definition %s. See -h for help.", meterdef)
			}
			metertype, devid := splitdef[0], splitdef[1]
			id, err := strconv.Atoi(devid)
			if err != nil {
				log.Fatalf("Error parsing device id %s: %s. See -h for help.", meterdef, err.Error())
			}
			metertype = strings.ToUpper(metertype)
			switch metertype {
			case sdm630.METER_JANITZA:
				meter = sdm630.NewMeter(uint8(id), sdm630.NewJanitzaRoundRobinScheduler())
			case sdm630.METER_SDM:
				meter = sdm630.NewMeter(uint8(id), sdm630.NewSDMRoundRobinScheduler())
			default:
				log.Fatalf("Unknown meter type %s for device %d. See -h for help.", metertype, id)
			}
			meters = append(meters, meter)
		}

		// Create Channels that link the goroutines
		var scheduler2queryengine = make(sdm630.QuerySnipChannel)
		var queryengine2duplicator = make(sdm630.QuerySnipChannel)
		var duplicator2cache = make(sdm630.QuerySnipChannel)
		var duplicator2firehose = make(sdm630.QuerySnipChannel)

		scheduler := sdm630.NewQueryScheduler(
			scheduler2queryengine,
			meters,
		)
		go scheduler.Run()

		qe := sdm630.NewModbusEngine(
			c.String("serialadapter"),
			c.Int("comset"),
			c.Bool("verbose"),
			status,
		)

		go qe.Transform(
			scheduler2queryengine,
			queryengine2duplicator,
		)

		// This is the duplicator
		go func(in sdm630.QuerySnipChannel,
			out1 sdm630.QuerySnipChannel,
			out2 sdm630.QuerySnipChannel,
		) {
			for {
				snip := <-in
				out1 <- snip
				out2 <- snip
			}
		}(queryengine2duplicator, duplicator2cache, duplicator2firehose)

		firehose := sdm630.NewFirehose(duplicator2firehose,
			c.Bool("verbose"))
		go firehose.Run()

		mc := sdm630.NewMeasurementCache(
			duplicator2cache,
			120*time.Second, // TODO: How long to store data in the cache?.
			c.Bool("verbose"),
		)
		go mc.Consume()

		log.Printf("Starting API httpd at %s", c.String("url"))
		sdm630.Run_httpd(
			mc,
			firehose,
			status,
			c.String("url"),
		)
	}

	app.Run(os.Args)
}
