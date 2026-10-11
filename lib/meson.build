versioned_libname = 'libapm-' + soversion
gir_name = 'Apm-' + soversion

deps_names = [
  'libserialize-7',
]

libapm_deps_conf = configuration_data()
libapm_deps_conf.set ('DEPS',  '\n'.join (deps_names))

libapm_dependencies = []

foreach dep : deps_names
  libapm_dependencies += dependency(dep)
endforeach

libapm_sources = files(
  'applications.vala',
  'client.vala',
  'codec.vala',
  'distrobox.vala',
  'icons.vala',
  'image.vala',
  'kernel.vala',
  'models/additional.vala',
  'models/distrobox/container-info.vala',
  'models/distrobox/list-response.vala',
  'models/distrobox/package-info.vala',
  'models/distrobox/package.vala',
  'models/distrobox/update-response.vala',
  'models/filter-fields-info.vala',
  'models/repo/add-remove-response.vala',
  'models/repo/info.vala',
  'models/repo/list-response.vala',
  'models/repo/set-response.vala',
  'models/repo/simulate-response.vala',
  'models/repo/task-packages-response.vala',
  'models/system/application/branding-info.vala',
  'models/system/application/content-rating-info.vala',
  'models/system/application/developer-info.vala',
  'models/system/application/display-length-info.vala',
  'models/system/application/icon-info.vala',
  'models/system/application/image-info.vala',
  'models/system/application/info-response.vala',
  'models/system/application/info.vala',
  'models/system/application/language-info.vala',
  'models/system/application/list-component-info.vala',
  'models/system/application/list-response.vala',
  'models/system/application/property-info.vala',
  'models/system/application/provides-dbus-info.vala',
  'models/system/application/provides-info.vala',
  'models/system/application/release-artifact-checksum-info.vala',
  'models/system/application/release-artifact-info.vala',
  'models/system/application/release-artifact-size-info.vala',
  'models/system/application/release-info.vala',
  'models/system/application/release-issue-info.vala',
  'models/system/application/release-url-info.vala',
  'models/system/application/screenshot-info.vala',
  'models/system/application/translation-info.vala',
  'models/system/application/update-response.vala',
  'models/system/apt-config-response.vala',
  'models/system/boot-image-info.vala',
  'models/system/check-essential-package-info.vala',
  'models/system/check-info.vala',
  'models/system/check-response.vala',
  'models/system/config/config.vala',
  'models/system/config/modules/base.vala',
  'models/system/config/modules/branding.vala',
  'models/system/config/modules/git.vala',
  'models/system/config/modules/kernel.vala',
  'models/system/config/modules/packages.vala',
  'models/system/config/modules/repos.vala',
  'models/system/image/apply-response.vala',
  'models/system/image/history-response.vala',
  'models/system/image/info.vala',
  'models/system/image/spec-image-info.vala',
  'models/system/image/spec-info.vala',
  'models/system/image/status-image-data.vala',
  'models/system/image/status-image-info.vala',
  'models/system/image/status-info.vala',
  'models/system/kernel/check-old-kernels-response.vala',
  'models/system/kernel/info.vala',
  'models/system/kernel/install-kernel-modules-response.vala',
  'models/system/kernel/install-update-kernel-response.vala',
  'models/system/kernel/list-kernel-modules-response.vala',
  'models/system/kernel/module-info.vala',
  'models/system/kernel/remove-kernel-modules-response.vala',
  'models/system/kernel/w-reasons.vala',
  'models/system/kernel/with-modules-info.vala',
  'models/system/list-response.vala',
  'models/system/multi-info-response.vala',
  'models/system/package-info.vala',
  'models/system/search-response.vala',
  'models/system/update-response.vala',
  'models/system/upgrade-response.vala',
  'packages.vala',
  'repo.vala',
  'transport.vala',
)

add_project_arguments(
  '-w',
  language: ['c'],
)

add_project_arguments(
  '--abi-stability',
  '--hide-internal',
  '--vapi-comments',
  language: ['vala'],
)

libapm = library(
  meson.project_name(),
  libapm_sources,
  dependencies: libapm_dependencies,
  soversion: soversion,
  version: meson.project_version(),
  vala_header: versioned_libname + '.h',
  vala_vapi: versioned_libname + '.vapi',
  vala_gir: gir_name + '.gir',
  vala_args: ['--target-glib=2.76', '--abi-stability', '--hide-internal', '--vapi-comments'],
  install: true,
  install_dir: [ true, true, true, true ],
)

libapm_dep = declare_dependency(
  link_with: libapm,
  include_directories: include_directories('.'),
  dependencies: libapm_dependencies,
)

configure_file(
  input: 'lib.deps.in',
  output: versioned_libname + '.deps',
  configuration: libapm_deps_conf,
  install: true,
  install_dir: get_option('datadir') / 'vala' / 'vapi',
)

if get_option('introspection')
  g_ir_compiler = find_program('g-ir-compiler')
  custom_target(
    'typelib',
    command: [
      g_ir_compiler,
      '--shared-library', '@PLAINNAME@',
      '--output', '@OUTPUT@',
      meson.current_build_dir() / gir_name + '.gir',
    ],
    input: libapm,
    output: gir_name + '.typelib',
    depends: libapm,
    install: true,
    install_dir: get_option('libdir') / 'girepository-1.0',
  )
endif

pkgconfig.generate(
  libapm,
  name: versioned_libname,
  requires: libapm_dependencies,
  subdirs: [ versioned_libname ],
  description: 'APM DBus library',
  version: meson.project_version(),
  url: 'https://altlinux.space/alt-atomic/apm',
)
