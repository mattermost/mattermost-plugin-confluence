import React from 'react';
import PropTypes from 'prop-types';
import {Form} from 'react-bootstrap';
import Select from 'react-select';

import {getStyleForReactSelect} from '../utils/react_select_styles';

export default class ConfluenceField extends React.PureComponent {
    static propTypes = {
        required: PropTypes.bool.isRequired,
        value: PropTypes.oneOfType([
            PropTypes.string,
            PropTypes.number,
            PropTypes.object,
            PropTypes.array,
        ]),
        label: PropTypes.string.isRequired,
        onChange: PropTypes.func.isRequired,
        addValidation: PropTypes.func.isRequired,
        removeValidation: PropTypes.func.isRequired,
        theme: PropTypes.object,
        fieldType: PropTypes.string.isRequired,
        readOnly: PropTypes.bool,
        formGroupStyle: PropTypes.object,
        formControlStyle: PropTypes.object,
        type: PropTypes.string,
        placeholder: PropTypes.string,
        name: PropTypes.string,
        options: PropTypes.array,
        isMulti: PropTypes.bool,
        isSearchable: PropTypes.bool,
        testId: PropTypes.string,
    };

    static defaultProps = {
        readOnly: false,
        formGroupStyle: {},
        formControlStyle: {},
    };

    constructor(props) {
        super(props);
        this.state = {
            valid: true,
        };
    }

    componentDidMount() {
        if (this.props.addValidation) {
            this.props.addValidation(this.isValid);
        }
    }

    handleChange = (e) => {
        if (!this.state.valid) {
            this.setState({
                valid: true,
            });
        }
        this.props.onChange(e);
    };

    componentWillUnmount() {
        if (this.props.removeValidation) {
            this.props.removeValidation(this.isValid);
        }
    }

    isValid = () => {
        const {fieldType, value, required} = this.props;
        if (required &&
            (value === null ||
            (typeof value === 'string' && !value.trim()) ||
            (fieldType === 'dropDown' && value.length === 0) ||
            !value)
        ) {
            this.setState({
                valid: false,
            });
            return false;
        }
        return true;
    };

    render() {
        const {
            required, fieldType, theme, label, formGroupStyle, formControlStyle,
            value, type, placeholder, name, readOnly, options, isMulti, isSearchable, testId,
        } = this.props;
        const requiredErrorMsg = 'This field is required.';
        let requiredError = null;
        if (required && !this.state.valid) {
            requiredError = (
                <p className='help-text error-text'>
                    <span>{requiredErrorMsg}</span>
                </p>
            );
        }
        let field = null;
        if (fieldType === 'input') {
            field = (
                <Form.Control
                    style={formControlStyle}
                    type={type}
                    placeholder={placeholder}
                    value={value}
                    readOnly={readOnly}
                    onChange={this.handleChange}
                    data-testid={testId}
                />
            );
        } else if (fieldType === 'dropDown') {
            field = (
                <Select
                    name={name}
                    value={value}
                    options={options}
                    isMulti={isMulti}
                    isSearchable={isSearchable}
                    menuPortalTarget={document.body}
                    menuPlacement='auto'
                    styles={getStyleForReactSelect(theme)}
                    onChange={this.handleChange}
                    inputId={testId}
                />
            );
        }
        return (
            <Form.Group style={formGroupStyle}>
                <Form.Label>{label}</Form.Label>
                {required &&
                <span
                    className='error-text'
                    style={{marginLeft: '3px'}}
                >
                    {'*'}
                </span> }
                {field}
                {requiredError}
            </Form.Group>
        );
    }
}
