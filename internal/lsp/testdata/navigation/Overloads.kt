package nav

import nav.Packet as PacketAlias

class Packet

fun parse(value: PacketAlias): Int = 1
fun parse(value: Int): Int = 2
fun PacketAlias.convert(): Int = 3
