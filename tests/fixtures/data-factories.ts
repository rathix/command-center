import { faker } from '@faker-js/faker';

export const createUserData = (overrides = {}) => ({
  name: faker.person.fullName(),
  email: faker.internet.email(),
  ...overrides,
});

export const createServiceData = (overrides = {}) => ({
  name: faker.commerce.productName(),
  url: faker.internet.url(),
  namespace: 'default',
  ...overrides,
});
