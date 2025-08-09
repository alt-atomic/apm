project('apm', 'cpp',
  version: '0.1.2',
  meson_version: '>= 1.0.0',
  default_options: [
    'warning_level=2',
    'werror=false',
    'cpp_std=c++17',
  ],
)

i18n = import('i18n')

go_root = run_command('printenv', 'GOROOT', check: false)
go_bin = go_root.returncode() != 0 ? find_program('go') : find_program(join_paths(go_root.stdout().strip(), 'bin', 'go'))

cpp = meson.get_compiler('cpp')
apt_dep = dependency('apt-pkg', required: false)

if not apt_dep.found()
  apt_lib = cpp.find_library('apt-pkg',
    dirs: ['/usr/lib64', '/usr/lib'],
    required: true,
  )
  apt_inc = include_directories('/usr/include/apt-pkg')

  apt_dep = declare_dependency(
    include_directories: apt_inc,
    dependencies: [apt_lib],
  )
endif

SERVICE_ID = 'org.altlinux.APM'

COMMAND_PREFIX = ''
PROFILE = get_option('profile')
LOCALES_DIR = get_option('prefix') / get_option('localedir')
LOG_FILEPATH = get_option('localstatedir') / 'lib' / meson.project_name() / meson.project_name() + '.log'
DBSQL_FILEPATH = get_option('localstatedir') / 'lib' / meson.project_name() / meson.project_name() + '.db'
IMAGE_FILEPATH = get_option('sysconfdir') / meson.project_name() / 'image.yml'

constants = [
  ['apm/lib.BuildCommandPrefix', COMMAND_PREFIX],
  ['apm/lib.BuildEnvironment', PROFILE],
  ['apm/lib.BuildPathLocales', LOCALES_DIR],
  ['apm/lib.BuildPathLogFile', LOG_FILEPATH],
  ['apm/lib.BuildPathDBSQLSystem', DBSQL_FILEPATH],
  ['apm/lib.BuildPathImageFile', IMAGE_FILEPATH],
]

ldflags = []
foreach constant : constants
  ldflags += '-X \'@0@=@1@\''.format(constant[0], constant[1])
endforeach

# --- C++ wrapper ---
apt_wrapper_sources = [
  'internal/common/binding/apt/wrapper/apt_wrapper.cpp',
  'internal/common/binding/apt/wrapper/apt_install.cpp',
  'internal/common/binding/apt/wrapper/apt_dist.cpp',
  'internal/common/binding/apt/wrapper/apt_simulate.cpp',
  'internal/common/binding/apt/wrapper/apt_search.cpp',
  'internal/common/binding/apt/wrapper/apt_getinfo.cpp',
]
apt_wrapper_inc = include_directories('internal/common/binding/apt/wrapper')

apt_wrapper_lib = static_library(
  'apt_wrapper',
  apt_wrapper_sources,
  include_directories: [apt_wrapper_inc],
  dependencies: [apt_dep],
  cpp_args: ['-DAPT_WRAPPER_BUILD', '-fPIC'],
)

subdir('po')
subdir('data')

custom_target(
  'go-build',
  build_by_default: true,
  build_always_stale: true,
  output: meson.project_name(),
  console: true,
  install: true,
  install_dir: get_option('bindir'),
  command: [
    go_bin, 'build', '-v',
    '-ldflags', ' '.join(ldflags),
    '-o', '@OUTPUT@',
    meson.current_source_dir()
  ],
  depends: [apt_wrapper_lib],
  env: [
    'CGO_ENABLED=1',
    'CGO_CFLAGS=' +
      '-I' + meson.current_source_dir() + '/internal/common/binding/apt/wrapper ' +
      '-I/usr/include/apt-pkg',
    'CGO_LDFLAGS=' +
      '-L' + meson.current_build_dir() + ' ' +
      '-Wl,-Bstatic -lapt_wrapper -Wl,-Bdynamic -lapt-pkg -lstdc++',
  ]
)
