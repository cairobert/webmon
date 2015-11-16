package snmp
import (
		"encoding/asn1"
	   )

const (
			CPU		= 0X1
			MEMORY	= 0X2
			DISK	= 0X3
	  )

var (
			CPU_RAW_USER		asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 11, 50, 0}
			CPU_RAW_NICE		asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 11, 51, 0}
			CPU_RAW_SYSTEM		asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 11, 52, 0}
			CPU_RAW_IDLE		asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 11, 53, 0}
			CPU_RAW_WAIT		asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 11, 54, 0}

	  		MEM_TOTAL			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 4, 5, 0}
			MEM_AVAIL			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 4, 6, 0}
			SWAP_TOTAL			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 4, 3, 0}
			SWAP_AVAIL			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 4, 4, 0}

			// type: table, to be modified
			DISK_DEVICE			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 9, 1, 3}
			DISK_PATH			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 9, 1, 2}
			DISK_TOTAL			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 9, 1, 6}
			DISK_AVAIL			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 4, 1, 2021, 9, 1, 7}
// TODO: add netbytes

			IF_DESCR			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 2, 1, 2, 2, 1, 2}
			IF_INOCTETS			asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 2, 1, 2, 2, 1, 10}
			IF_OUTOCTETS		asn1.ObjectIdentifier	= []int{1, 3, 6, 1, 2, 1, 2, 2, 1, 16}
	  )

