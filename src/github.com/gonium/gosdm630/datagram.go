package sdm630

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"time"
)

// UniqueIdFormat is a format string for unique ID generation.
// It expects one %d conversion specifier,
// which will be replaced with the device ID.
// The UniqueIdFormat can be changed on program startup,
// before any additional goroutines are started.
var UniqueIdFormat string = "Instrument%d"

/***
 * This is the definition of the Reading datatype. It combines readings
 * of all measurements into one data structure
 */

type ReadingChannel chan Readings

type Readings struct {
	UniqueId       string
	Timestamp      time.Time
	Unix           int64
	ModbusDeviceId uint8
	Power          ThreePhaseReadings
	Voltage        ThreePhaseReadings
	Current        ThreePhaseReadings
	Cosphi         ThreePhaseReadings
	Import         ThreePhaseReadings
	TotalImport    *float64
	Export         ThreePhaseReadings
	TotalExport    *float64
	THD            struct {
		//	Current           ThreePhaseReadings
		//	AvgCurrent        float64
		VoltageNeutral    ThreePhaseReadings
		AvgVoltageNeutral *float64
	}
}

type ThreePhaseReadings struct {
	L1 *float64
	L2 *float64
	L3 *float64
}

// Helper: Converts float64 to *float64
func F2fp(x float64) *float64 {
	return &x
}

// Helper: Converts *float64 to float64, correctly handles uninitialized
// variables
func Fp2f(x *float64) float64 {
	if x == nil {
		// this is not initialized yet - return NaN
		return math.Log(-1.0)
	} else {
		return *x
	}
}

func (r *Readings) String() string {
	fmtString := "UniqueId: %s ID: %d T: %s - L1: %.2fV %.2fA %.2fW %.2fcos | " +
		"L2: %.2fV %.2fA %.2fW %.2fcos | " +
		"L3: %.2fV %.2fA %.2fW %.2fcos"
	return fmt.Sprintf(fmtString,
		r.UniqueId,
		r.ModbusDeviceId,
		r.Timestamp.Format(time.RFC3339),
		Fp2f(r.Voltage.L1),
		Fp2f(r.Current.L1),
		Fp2f(r.Power.L1),
		Fp2f(r.Cosphi.L1),
		Fp2f(r.Voltage.L2),
		Fp2f(r.Current.L2),
		Fp2f(r.Power.L2),
		Fp2f(r.Cosphi.L2),
		Fp2f(r.Voltage.L3),
		Fp2f(r.Current.L3),
		Fp2f(r.Power.L3),
		Fp2f(r.Cosphi.L3),
	)
}

func (r *Readings) JSON(w io.Writer) error {
	return json.NewEncoder(w).Encode(r)
}

/*
 * Returns true if the reading is older than the given timestamp.
 */
func (r *Readings) IsOlderThan(ts time.Time) (retval bool) {
	return r.Timestamp.Before(ts)
}

/*
* Adds two readings. The individual values are added except for
* the time: the latter of the two times is copied over to the result
 */
func (lhs *Readings) add(rhs *Readings) (retval Readings, err error) {
	if lhs.ModbusDeviceId != rhs.ModbusDeviceId {
		return Readings{}, fmt.Errorf(
			"Cannot add readings of different devices - got IDs %d and %d",
			lhs.ModbusDeviceId, rhs.ModbusDeviceId)
	} else {
		retval = Readings{
			UniqueId:       lhs.UniqueId,
			ModbusDeviceId: lhs.ModbusDeviceId,
			Voltage: ThreePhaseReadings{
				L1: F2fp((*lhs.Voltage.L1) + (*rhs.Voltage.L1)),
				L2: F2fp((*lhs.Voltage.L2) + (*rhs.Voltage.L2)),
				L3: F2fp((*lhs.Voltage.L3) + (*rhs.Voltage.L3)),
			},
			Current: ThreePhaseReadings{
				L1: F2fp((*lhs.Current.L1) + (*rhs.Current.L1)),
				L2: F2fp((*lhs.Current.L2) + (*rhs.Current.L2)),
				L3: F2fp((*lhs.Current.L3) + (*rhs.Current.L3)),
			},
			Power: ThreePhaseReadings{
				L1: F2fp((*lhs.Power.L1) + (*rhs.Power.L1)),
				L2: F2fp((*lhs.Power.L2) + (*rhs.Power.L2)),
				L3: F2fp((*lhs.Power.L3) + (*rhs.Power.L3)),
			},
			Cosphi: ThreePhaseReadings{
				L1: F2fp((*lhs.Cosphi.L1) + (*rhs.Cosphi.L1)),
				L2: F2fp((*lhs.Cosphi.L2) + (*rhs.Cosphi.L2)),
				L3: F2fp((*lhs.Cosphi.L3) + (*rhs.Cosphi.L3)),
			},
		}
		if lhs.Timestamp.After(rhs.Timestamp) {
			retval.Timestamp = lhs.Timestamp
			retval.Unix = lhs.Unix
		} else {
			retval.Timestamp = rhs.Timestamp
			retval.Unix = rhs.Unix
		}
		return retval, nil
	}
}

/*
* Dive a reading by an integer. The individual values are divided except
* for the time: it is simply copied over to the result
 */
func (lhs *Readings) divide(scalar float64) (retval Readings) {
	retval = Readings{
		Voltage: ThreePhaseReadings{
			L1: F2fp(*lhs.Voltage.L1 / scalar),
			L2: F2fp(*lhs.Voltage.L2 / scalar),
			L3: F2fp(*lhs.Voltage.L3 / scalar),
		},
		Current: ThreePhaseReadings{
			L1: F2fp(*lhs.Current.L1 / scalar),
			L2: F2fp(*lhs.Current.L2 / scalar),
			L3: F2fp(*lhs.Current.L3 / scalar),
		},
		Power: ThreePhaseReadings{
			L1: F2fp(*lhs.Power.L1 / scalar),
			L2: F2fp(*lhs.Power.L2 / scalar),
			L3: F2fp(*lhs.Power.L3 / scalar),
		},
		Cosphi: ThreePhaseReadings{
			L1: F2fp(*lhs.Cosphi.L1 / scalar),
			L2: F2fp(*lhs.Cosphi.L2 / scalar),
			L3: F2fp(*lhs.Cosphi.L3 / scalar),
		},
	}
	retval.Timestamp = lhs.Timestamp
	retval.Unix = lhs.Unix
	retval.ModbusDeviceId = lhs.ModbusDeviceId
	retval.UniqueId = lhs.UniqueId
	return retval
}

/* ReadingSlice is a type alias for a slice of readings.
 */
type ReadingSlice []Readings

func (r ReadingSlice) JSON(w io.Writer) error {
	return json.NewEncoder(w).Encode(r)
}

func (r ReadingSlice) NotOlderThan(ts time.Time) (retval ReadingSlice) {
	retval = ReadingSlice{}
	for _, reading := range r {
		if !reading.IsOlderThan(ts) {
			retval = append(retval, reading)
		}
	}
	return retval
}

/***
 * A QuerySnip is just a little snippet of query information. It
 * encapsulates modbus query targets.
 */

type QuerySnip struct {
	DeviceId      uint8
	FuncCode      uint8
	OpCode        uint16 `json:"-"`
	Value         float64
	IEC61850      string
	Description   string
	ReadTimestamp time.Time
}

type QuerySnipChannel chan QuerySnip

/**
 * MergeSnip adds the values represented by the QuerySnip to the
 * Readings. It also updates the current time stamp with the actual
 * time.
 */
func (r *Readings) MergeSnip(q QuerySnip) {
	r.Timestamp = q.ReadTimestamp
	r.Unix = r.Timestamp.Unix()
	switch q.IEC61850 {
	case "VolLocPhsA":
		r.Voltage.L1 = &q.Value
	case "VolLocPhsB":
		r.Voltage.L2 = &q.Value
	case "VolLocPhsC":
		r.Voltage.L3 = &q.Value
	case "AmpLocPhsA":
		r.Current.L1 = &q.Value
	case "AmpLocPhsB":
		r.Current.L2 = &q.Value
	case "AmpLocPhsC":
		r.Current.L3 = &q.Value
	case "WLocPhsA":
		r.Power.L1 = &q.Value
	case "WLocPhsB":
		r.Power.L2 = &q.Value
	case "WLocPhsC":
		r.Power.L3 = &q.Value
	case "AngLocPhsA":
		r.Cosphi.L1 = &q.Value
	case "AngLocPhsB":
		r.Cosphi.L2 = &q.Value
	case "AngLocPhsC":
		r.Cosphi.L3 = &q.Value
	case "TotkWhImportPhsA":
		r.Import.L1 = &q.Value
	case "TotkWhImportPhsB":
		r.Import.L2 = &q.Value
	case "TotkWhImportPhsC":
		r.Import.L3 = &q.Value
	case "TotkWhImport":
		r.TotalImport = &q.Value
	case "TotkWhExportPhsA":
		r.Export.L1 = &q.Value
	case "TotkWhExportPhsB":
		r.Export.L2 = &q.Value
	case "TotkWhExportPhsC":
		r.Export.L3 = &q.Value
	case "TotkWhExport":
		r.TotalExport = &q.Value
		//	case OpCodeL1THDCurrent:
		//		r.THD.Current.L1 = &q.Value
		//	case OpCodeL2THDCurrent:
		//		r.THD.Current.L2 = &q.Value
		//	case OpCodeL3THDCurrent:
		//		r.THD.Current.L3 = &q.Value
		//	case OpCodeAvgTHDCurrent:
		//		r.THD.AvgCurrent = &q.Value
	case "ThdVolPhsA":
		r.THD.VoltageNeutral.L1 = &q.Value
	case "ThdVolPhsB":
		r.THD.VoltageNeutral.L2 = &q.Value
	case "ThdVolPhsC":
		r.THD.VoltageNeutral.L3 = &q.Value
	case "ThdVol":
		r.THD.AvgVoltageNeutral = &q.Value
	default:
		log.Fatalf("Cannot merge unknown snip type - snip is %+v", q)
	}

}

func (q QuerySnip) String() string {
	return fmt.Sprintf("DevID: %d, FunCode: %d, Opcode %x: Value: %.3f",
		q.DeviceId, q.FuncCode, q.OpCode, q.Value)
}
