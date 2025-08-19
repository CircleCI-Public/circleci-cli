require 'aruba/cucumber'

Before('@k9s') do
  token = ENV['K9S_CIRCLECI_CLI_TOKEN']
  ENV['CIRCLECI_CLI_TOKEN'] = token if token && !token.empty?
end

After('@k9s') do
  ENV.delete('CIRCLECI_CLI_TOKEN')
end
