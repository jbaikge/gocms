import * as React from 'react';
import { Admin, Resource } from 'react-admin';
import { ClassCreate, ClassEdit, ClassList } from './classes';
import dataProvider from './dataProvider';
import { darkTheme } from './theme';

// import jsonServerProvider from 'ra-data-json-server';
// const dataProvider = jsonServerProvider('https://jsonplaceholder.typicode.com');

const App = () => (
  <Admin dataProvider={dataProvider} theme={darkTheme}>
    <Resource name="classes" create={ClassCreate} edit={ClassEdit} list={ClassList} />
  </Admin>
);

export default App;
