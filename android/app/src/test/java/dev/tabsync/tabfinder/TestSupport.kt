package dev.tabsync.tabfinder

import dev.tabsync.tabfinder.data.Finder
import dev.tabsync.tabfinder.data.RootStore
import java.io.ByteArrayOutputStream
import java.io.File
import java.nio.file.Files
import org.junit.rules.TemporaryFolder

/** The host build of tabscan, which Gradle builds before the tests run (see build.gradle.kts). */
fun hostTabscan(): File = File(checkNotNull(System.getProperty("tabscan.binary")) { "tabscan.binary not set" }).also { check(it.canExecute()) { "$it is not built" } }

class MemoryRootStore(override var root: String? = null) : RootStore

/** A TuxGuitar 1 file with just its header: title, artist and album (UTF-16BE, length-prefixed). */
fun tg1(title: String, artist: String, album: String): ByteArray {
  val out = ByteArrayOutputStream()
  for (s in listOf("TuxGuitar File Format - 1.5", title, artist, album)) {
    out.write(s.length)
    out.write(s.toByteArray(Charsets.UTF_16BE))
  }
  return out.toByteArray()
}

fun TemporaryFolder.tree(vararg files: Pair<String, ByteArray>): File {
  val root = newFolder("tabs")
  for ((path, data) in files) File(root, path).apply { parentFile.mkdirs() }.writeBytes(data)
  return root
}

/** A shell script standing in for tabscan. */
fun TemporaryFolder.fakeBinary(script: String): File =
  newFile("fake-tabscan-${System.nanoTime()}").apply {
    writeText("#!/bin/sh\n$script\n")
    setExecutable(true)
  }

/** A Finder over the host tabscan, with its data in a temp dir. */
class Backend(folder: TemporaryFolder, binary: File = hostTabscan(), val store: MemoryRootStore = MemoryRootStore()) {
  val dataDir: File = folder.newFolder("data")
  val index = File(dataDir, "index.jsonl")
  val finder = Finder(binary, dataDir, store)
}

fun isRoot(): Boolean = System.getProperty("user.name") == "root"

/** Makes a folder unreadable; the caller restores it. */
fun File.lock() = Files.setPosixFilePermissions(toPath(), emptySet())

fun File.unlock() = Files.setPosixFilePermissions(toPath(), java.nio.file.attribute.PosixFilePermissions.fromString("rwxr-xr-x"))
