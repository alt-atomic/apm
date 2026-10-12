valadoc_prog = find_program('valadoc')

libapm_doc_packages = []
foreach package : deps_names
  libapm_doc_packages += '--pkg=' + package
endforeach

libapm_valadoc = custom_target(
  'libapm_valadoc',
  command: [valadoc_prog] + libapm_doc_packages + [
    libapm_sources,
    '--target-glib=2.76',
    '--package-name=' + versioned_libname,
    '--package-version=' + meson.project_version(),
    '--verbose',
    '--force',
    '-o', '@OUTPUT@',
  ],
  build_by_default: true,
  input: libapm_sources,
  output: 'apm',
  depends: libapm,
)

libapm_devhelp = custom_target(
  'libapm_devhelp',
  command: [valadoc_prog] + libapm_doc_packages + [
    libapm_sources,
    '--doclet=devhelp',
    '--target-glib=2.76',
    '--package-name=' + versioned_libname,
    '--package-version=' + meson.project_version(),
    '--verbose',
    '--force',
    '-o', '@OUTPUT@',
  ],
  build_by_default: true,
  input: libapm_sources,
  output: 'apm-devhelp',
  depends: libapm,
)

doc_dir = get_option('prefix') / get_option('datadir') / 'doc' / versioned_libname
meson.add_install_script(
  'sh', '-c',
  'mkdir -p "${DESTDIR}@0@" && cp -r "@1@/." "${DESTDIR}@0@/" && sed -i "s/version=\"\"/version=\"2\"/" "${DESTDIR}@0@/@2@.devhelp2"'.format(
    doc_dir,
    meson.current_build_dir() / 'apm-devhelp' / versioned_libname,
    versioned_libname,
  ),
)
