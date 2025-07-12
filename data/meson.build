conf = configuration_data()
conf.set('SERVICE_ID', SERVICE_ID)
conf.set('USER', 'root')
conf.set('BINDIR', get_option('prefix') / get_option('bindir'))
conf.set('NAME', meson.project_name())

configure_file(
  input: SERVICE_ID + '.policy.in',
  output: SERVICE_ID + '.policy',
  configuration: conf,
  install: true,
  install_dir: join_paths(get_option('datadir'), 'polkit-1', 'actions')
)

configure_file(
  input: SERVICE_ID + '.conf.in',
  output: SERVICE_ID + '.conf',
  configuration: conf,
  install: true,
  install_dir: get_option('sysconfdir') / 'dbus-1' / 'system.d'
)

configure_file(
  input: SERVICE_ID + '.service.in',
  output: SERVICE_ID + '.service',
  configuration: conf,
  install: true,
  install_dir: get_option('prefix') / get_option('datadir') / 'dbus-1' / 'system-services'
)

configure_file(
  input: SERVICE_ID + '.User.service.in',
  output: SERVICE_ID + '.User.service',
  configuration: conf,
  install: true,
  install_dir: get_option('prefix') / get_option('datadir') / 'dbus-1' / 'services'
)

systemd = dependency('systemd')
systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')

configure_file(
  input: meson.project_name() + '.service.in',
  output: meson.project_name() + '.service',
  configuration: conf,
  install: true,
  install_dir: systemd_system_unit_dir
)
