#!/usr/bin/env bash
# gen.sh writes a module of N files of Moshi models into DIR/m, for timing
# Moshi's codegen on more than test/moshi's one file:
#
#	bench/ksp/moshi/gen.sh DIR [N]
#
# N (default 100) is the number of ModelNNN.kt files. Each has four
# @JsonClass(generateAdapter = true) classes, in the shapes test/moshi
# has: a data class with nullable, defaulted, renamed (@Json), list, map
# and enum properties; a generic page of it; a class with var and
# @Transient properties; and one nesting them. Each file but every tenth
# also uses the models of the tenth before it (Model000 for Model001 to
# Model009), as a module's models refer to shared ones. The output only
# depends on N.
set -euo pipefail

[ $# -ge 1 ] || { sed -n '2,14p' "$0" >&2; exit 2; }
dir=$1 n=${2:-100}
rm -rf "$dir"
mkdir -p "$dir/m"

for ((i = 0; i < n; i++)); do
	F=$(printf 'Model%03d' "$i")
	if ((i % 10)); then
		H=$(printf 'Model%03d' $((i / 10 * 10)))
		shared="
    val owner: ${H}User,
    val related: List<${H}Page<${H}User>> = emptyList(),"
	else
		shared=""
	fi
	cat > "$dir/m/$F.kt" <<EOF
package m

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

enum class ${F}Role { ADMIN, MEMBER, GUEST }

@JsonClass(generateAdapter = true)
data class ${F}User(
    val id: Long,
    val name: String,
    @Json(name = "e_mail") val email: String?,
    val age: Int = $i,
    val tags: List<String> = emptyList(),
    val attributes: Map<String, String> = emptyMap(),
    val role: ${F}Role = ${F}Role.MEMBER,
    val score: Double? = null,
)

@JsonClass(generateAdapter = true)
data class ${F}Page<T>(val items: List<T>, val next: String?, val total: Int = 0)

@JsonClass(generateAdapter = true)
class ${F}Settings(val flags: Map<String, Boolean>, val limits: IntArray) {
    var theme: String = "light"
    var version: Long = $i
    @Transient var cache: Any? = null
}

@JsonClass(generateAdapter = true)
data class ${F}Feed(
    val user: ${F}User,
    val pages: List<${F}Page<${F}User>>,
    val settings: ${F}Settings?,$shared
    @Json(name = "updated_at") val updatedAt: Long = 0L,
)
EOF
done
