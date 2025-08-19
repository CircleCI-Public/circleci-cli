require 'aruba/cucumber'

# For the @k9s scenario, inject an auth token from CI without ever printing it
Before('@k9s') do
  token = ENV['K9S_CIRCLECI_CLI_TOKEN']
  ENV['CIRCLECI_CLI_TOKEN'] = token if token && !token.empty?
end

# Ensure we don't leak the token in any output after the scenario
After('@k9s') do
  ENV.delete('CIRCLECI_CLI_TOKEN')
end
