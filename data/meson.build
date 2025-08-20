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

# Create log directory and an empty log file at install time
log_dir = get_option('localstatedir') / 'log' / meson.project_name()
install_emptydir(log_dir,install_mode: ['rwxrwsr-x', 'root', 'users'])

empty_log = custom_target(
  'empty-log-file',
  output: meson.project_name() + '.log',
  command: ['sh', '-lc', ': > @OUTPUT@ && chmod 0664 @OUTPUT@'],
  build_by_default: true,
  install: true,
  install_dir: log_dir,
  install_mode: ['rw-rw-r--', 'root', 'users']
)

# --- Shell completions ---
bash_comp_dep = dependency('bash-completion', required: false)
bash_completion_dir = bash_comp_dep.found() ? bash_comp_dep.get_variable(pkgconfig: 'completionsdir') : (get_option('datadir') / 'bash-completion' / 'completions')
install_data(
  'completion/bash/apm',
  install_dir: bash_completion_dir,
  rename: 'apm'
)

fish_completion_dir = get_option('datadir') / 'fish' / 'vendor_completions.d'
install_data(
  'completion/fish/apm.fish',
  install_dir: fish_completion_dir
)

zsh_completion_dir = get_option('datadir') / 'zsh' / 'site-functions'
install_data(
  'completion/zsh/_apm',
  install_dir: zsh_completion_dir
)
