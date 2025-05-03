// Mock for react-select
import React from 'react';

const Select = (props) => {
  return React.createElement('div', { className: 'mock-react-select' }, props.placeholder || 'Select');
};

export default Select;