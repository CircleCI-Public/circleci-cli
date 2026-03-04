require 'aruba/cucumber'

Before('@k9s') do
  token = ENV['K9S_CIRCLECI_CLI_TOKEN']
  skip_this_scenario('K9S_CIRCLECI_CLI_TOKEN is not set') if token.nil? || token.empty?

  ENV['CIRCLECI_CLI_TOKEN'] = token
end

After('@k9s') do
  ENV.delete('CIRCLECI_CLI_TOKEN')
end
