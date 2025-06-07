import org.scalajs.linker.interface.ModuleSplitStyle

ThisBuild / scalaVersion := "3.7.1"
ThisBuild / version := "0.1.0-SNAPSHOT"
ThisBuild / organization := "com.juldarigi"

lazy val root = project
  .in(file("."))
  .enablePlugins(ScalaJSPlugin)
  .settings(
    name := "juldarigi",
    
    // Scala.js configuration
    scalaJSUseMainModuleInitializer := true,
    
    // Dependencies
    libraryDependencies ++= Seq(
      "org.scala-js" %%% "scalajs-dom" % "2.8.0",
      "com.raquo" %%% "laminar" % "17.0.0"
    ),
    
    // Output settings for Vite plugin
    scalaJSLinkerConfig ~= {
      _.withModuleKind(ModuleKind.ESModule)
        .withSourceMap(false)
        .withModuleSplitStyle(
          ModuleSplitStyle.SmallModulesFor(List("juldarigi"))
        )
    }
  )

// Custom task for continuous compilation
addCommandAlias("dev", "~fastLinkJS")