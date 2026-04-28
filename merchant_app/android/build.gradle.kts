allprojects {
    repositories {
        google()
        mavenCentral()
        // 小米推送
        maven { url = uri("https://sdk.it.mi.com/maven") }
        // 荣耀推送
        maven { url = uri("https://developer.hihonor.com/repo") }
    }
}

val newBuildDir: Directory =
    rootProject.layout.buildDirectory
        .dir("../../build")
        .get()
rootProject.layout.buildDirectory.value(newBuildDir)

subprojects {
    val newSubprojectBuildDir: Directory = newBuildDir.dir(project.name)
    project.layout.buildDirectory.value(newSubprojectBuildDir)
}
subprojects {
    project.evaluationDependsOn(":app")
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
