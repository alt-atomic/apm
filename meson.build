project('apm', 'cpp',
  version: '0.1.11',
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

version = run_command(
  'sh',
  meson.current_source_dir() + '/scripts/get-version.sh',
  meson.project_version(),
  check: false
).stdout()

cpp = meson.get_compiler('cpp')
apt_lib = cpp.find_library('apt-pkg', dirs: ['/usr/lib64', '/usr/lib'], required: true)
apt_inc = include_directories('/usr/include/apt-pkg')
apt_dep = declare_dependency(include_directories: apt_inc, dependencies: [apt_lib])

SERVICE_ID = 'org.altlinux.APM'

COMMAND_PREFIX = ''
PROFILE = get_option('profile')
LOCALES_DIR = get_option('prefix') / get_option('localedir')
DBSQL_FILEPATH = get_option('localstatedir') / 'lib' / meson.project_name() / meson.project_name() + '.db'
IMAGE_FILEPATH = get_option('sysconfdir') / meson.project_name() / 'image.yml'
RESOURCES_FILEPATH = get_option('sysconfdir') / meson.project_name() / 'resources'

constants = [
  ['apm/internal/common/app.BuildCommandPrefix', COMMAND_PREFIX],
  ['apm/internal/common/app.BuildEnvironment', PROFILE],
  ['apm/internal/common/app.BuildPathLocales', LOCALES_DIR],
  ['apm/internal/common/app.BuildPathDBSQLSystem', DBSQL_FILEPATH],
  ['apm/internal/common/app.BuildPathImageFile', IMAGE_FILEPATH],
  ['apm/internal/common/app.BuildPathResourcesDir', RESOURCES_FILEPATH],
  ['apm/internal/common/app.BuildVersion', version],
]

ldflags = []
foreach constant : constants
  ldflags += '-X \'@0@=@1@\''.format(constant[0], constant[1])
endforeach

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
    go_bin,
    'build', '-v',
    '-ldflags', ' '.join(ldflags),
    '-o', '@OUTPUT@',
    meson.current_source_dir()
  ],
  env: [
    'CGO_ENABLED=1',
  ]
)
